package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

func main() {
	// 加载配置
	config, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("无法加载配置: %v", err)
	}
	// 初始化日志文件
	logFile, err := os.OpenFile(config.LogDirectory, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("无法创建日志文件:", err)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime)

	// 注册路由
	http.HandleFunc("/chat/completions", openAIProxyHandler)

	fmt.Printf("SSE proxy server started on %s\n", config.ServerAddress)
	log.Fatal(http.ListenAndServe(config.ServerAddress, nil))
}

// openAIProxyHandler 处理OpenAI格式的请求并转发到私有API
func openAIProxyHandler(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// 处理OPTIONS请求（预检请求）
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1. 验证请求方法
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. 读取请求体
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("读取请求体失败: %v", err)
		http.Error(w, "无法读取请求", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	// 3. 解析OpenAI请求
	var openAIReq OpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		log.Printf("解析OpenAI请求失败: %v", err)
		http.Error(w, "无效的请求格式", http.StatusBadRequest)
		return
	}

	// 记录输入消息
	log.Printf("收到请求消息: %+v\n", openAIReq.Messages)

	// 4. 转换为私有API格式
	config := GetConfig()
	privateReq, err := ConvertOpenAIToPrivateAPI(&openAIReq, config)
	if err != nil {
		log.Printf("转换请求失败: %v", err)
		http.Error(w, fmt.Sprintf("转换请求失败: %v", err), http.StatusBadRequest)
		return
	}

	// 5. 序列化为JSON
	privateReqBody, err := json.Marshal(privateReq)
	if err != nil {
		log.Printf("序列化请求失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 6. 构建到目标API的请求
	targetURL, err := url.Parse(config.TargetBaseURL)
	if err != nil {
		log.Printf("解析目标URL失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 创建新请求
	proxyReq, err := http.NewRequest(http.MethodPost, targetURL.String(), bytes.NewBuffer(privateReqBody))
	if err != nil {
		log.Printf("创建目标请求失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 设置请求头
	proxyReq.Header.Set("Content-Type", "application/json")

	// 7. 发送请求到目标API
	client := &http.Client{}
	targetResp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("连接到目标API失败: %v", err)
		http.Error(w, "连接到目标服务失败", http.StatusBadGateway)
		return
	}
	defer targetResp.Body.Close()

	// 8. 检查目标响应状态码
	if targetResp.StatusCode != http.StatusOK {
		log.Printf("目标API返回非200状态: %d", targetResp.StatusCode)
		respBody, _ := ioutil.ReadAll(targetResp.Body)
		http.Error(w, fmt.Sprintf("目标返回状态 %d: %s", targetResp.StatusCode, string(respBody)), targetResp.StatusCode)
		return
	}

	// 9. 根据Stream参数决定响应方式
	if openAIReq.Stream {
		// SSE 流式响应
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.(http.Flusher).Flush()

		scanner := bufio.NewScanner(targetResp.Body)
		sessionId := uuid.New().String()
		created := time.Now().Unix()

		// 用于累积完整的响应消息
		var fullResponse strings.Builder

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			if strings.HasPrefix(line, "data:") {
				var privateResp PrivateAPISSEResponse
				jsonData := strings.TrimPrefix(line, "data:")
				if err := json.Unmarshal([]byte(jsonData), &privateResp); err != nil {
					log.Printf("解析私有API响应失败: %v", err)
					continue
				}

				// 累积响应消息
				fullResponse.WriteString(privateResp.ResponseMessage)

				openAIResp := OpenAISSEResponse{
					ID:                sessionId,
					Object:            "chat.completion.chunk",
					Created:           created,
					Model:             "deepseek-chat",
					SystemFingerprint: "fp_8802369eaa_prod0425fp8",
					Choices: []Choice{
						{
							Index: 0,
							Delta: Delta{
								Content: privateResp.ResponseMessage,
							},
							LogProbs:     nil,
							FinishReason: nil,
						},
					},
				}

				openAIRespJSON, err := json.Marshal(openAIResp)
				if err != nil {
					log.Printf("序列化OpenAI响应失败: %v", err)
					continue
				}

				fmt.Fprintf(w, "data: %s\n\n", string(openAIRespJSON))
				w.(http.Flusher).Flush()
			}
		}

		// 在流式响应结束时记录完整的消息
		if fullResponse.Len() > 0 {
			log.Printf("完整流式响应: %s", fullResponse.String())
		}

		if err := scanner.Err(); err != nil {
			log.Printf("SSE流读取过程中发生错误: %v", err)
		}

		log.Println("SSE代理连接关闭")
	} else {
		// 普通HTTP JSON响应
		w.Header().Set("Content-Type", "application/json")

		respBody, err := ioutil.ReadAll(targetResp.Body)
		if err != nil {
			log.Printf("读取目标响应失败: %v", err)
			http.Error(w, "内部服务器错误", http.StatusInternalServerError)
			return
		}

		var privateResp PrivateAPISSEResponse
		if err := json.Unmarshal(respBody, &privateResp); err != nil {
			log.Printf("解析私有API响应失败: %v", err)
			http.Error(w, "解析响应失败", http.StatusInternalServerError)
			return
		}

		// 记录非流式输出消息
		log.Printf("完整响应消息: %s\n", privateResp.ResponseMessage)

		// 计算token数量（这里使用简单的估算方法）
		promptTokens := 13                                                       // 示例值
		completionTokens := len(strings.Split(privateResp.ResponseMessage, " ")) // 粗略估算
		totalTokens := promptTokens + completionTokens

		openAIResp := OpenAISSEResponse{
			ID:                uuid.New().String(),
			Object:            "chat.completion",
			Created:           time.Now().Unix(),
			Model:             "deepseek-chat",
			SystemFingerprint: "fp_8802369eaa_prod0425fp8",
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: privateResp.ResponseMessage,
					},
					LogProbs:     nil,
					FinishReason: func() *string { s := "stop"; return &s }(),
				},
			},
			Usage: Usage{
				PromptTokens:          promptTokens,
				CompletionTokens:      completionTokens,
				TotalTokens:           totalTokens,
				PromptTokensDetails:   PromptTokensDetails{CachedTokens: 0},
				PromptCacheHitTokens:  0,
				PromptCacheMissTokens: promptTokens,
			},
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(openAIResp); err != nil {
			log.Printf("序列化响应失败: %v", err)
			http.Error(w, "内部服务器错误", http.StatusInternalServerError)
			return
		}
	}
}
