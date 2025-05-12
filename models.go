package main

import (
	"errors"
)

// OpenAI格式的SSE响应
type OpenAISSEResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,omitempty"`
	Delta        Delta   `json:"delta,omitempty"`
	LogProbs     *string `json:"logprobs"`
	FinishReason *string `json:"finish_reason"`
}

type Delta struct {
	Content string `json:"content"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens          int                `json:"prompt_tokens"`
	CompletionTokens      int                `json:"completion_tokens"`
	TotalTokens           int                `json:"total_tokens"`
	PromptTokensDetails   PromptTokensDetails `json:"prompt_tokens_details"`
	PromptCacheHitTokens  int                `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int                `json:"prompt_cache_miss_tokens"`
}

type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// 私有API的SSE响应
type PrivateAPISSEResponse struct {
	Success         bool        `json:"success"`
	Code            int         `json:"code"`
	ErrorMessage    *string     `json:"errorMessage"`
	ErrorDetail     *string     `json:"errorDetail"`
	RequestId       *string     `json:"requestId"`
	ResponseId      *string     `json:"responseId"`
	ResponseMessage string      `json:"responseMessage"`
	Data            interface{} `json:"data"`
}
// OpenAI请求模型
type OpenAIRequest struct {
	Model    string        `json:"model"`
	Messages []OpenAIInput `json:"messages"`
	Stream   bool          `json:"stream"`
}

// OpenAI输入项
type OpenAIInput struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // 可以是字符串或内容数组
}

// OpenAI内容项
type OpenAIContentItem struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"`
}

// OpenAI图片URL
type OpenAIImageURL struct {
	URL string `json:"url"`
}

// 私有API请求模型 - 文本请求
type PrivateAPITextRequest struct {
	AgentID     string              `json:"agentId"`
	SecretKey   string              `json:"secretKey"`
	ServiceName string              `json:"serviceName"`
	Messages    []PrivateAPIMessage `json:"messages"`
	Stream      bool                `json:"stream"`
}

// 私有API请求模型 - 多模态请求
type PrivateAPIMultiModalRequest struct {
	AgentID     string              `json:"agentId"`
	SecretKey   string              `json:"secretKey"`
	ServiceName string              `json:"serviceName"`
	Stream      bool                `json:"stream"`
	Messages    []PrivateAPIMessage `json:"messages"`
}

// 私有API消息
type PrivateAPIMessage struct {
	Role    string                  `json:"role"`
	Content []PrivateAPIContentItem `json:"content"`
}

// 私有API内容项
type PrivateAPIContentItem struct {
	Type     string              `json:"type"`
	Text     string              `json:"text,omitempty"`
	ImageURL *PrivateAPIImageURL `json:"image_url,omitempty"`
}

// 私有API图片URL
type PrivateAPIImageURL struct {
	URL string `json:"url"`
}

// ConvertOpenAIToPrivateAPI 将OpenAI请求转换为私有API请求
func ConvertOpenAIToPrivateAPI(openAIReq *OpenAIRequest, config *Config) (interface{}, error) {
	if openAIReq == nil {
		return nil, errors.New("无效的OpenAI请求")
	}

	// 获取模型名称映射，如果存在映射则使用映射后的服务名称，否则保持原样
	serviceName := openAIReq.Model
	if config.ModelMap != nil {
		if mappedName, ok := config.ModelMap[openAIReq.Model]; ok {
			serviceName = mappedName
		}
	}

	// 检查是否包含图片内容
	hasImage := false
	for _, message := range openAIReq.Messages {
		// 检查内容是否为数组
		contentArray, isArray := message.Content.([]interface{})
		if isArray {
			for _, item := range contentArray {
				if contentMap, ok := item.(map[string]interface{}); ok {
					if contentType, ok := contentMap["type"].(string); ok && contentType == "input_image" {
						hasImage = true
						break
					}
				}
			}
		}
		if hasImage {
			break
		}
	}

	// 根据是否有图片选择不同的请求格式
	if hasImage {
		return convertToMultiModalRequest(openAIReq, serviceName, config)
	} else {
		return convertToTextRequest(openAIReq, serviceName, config)
	}
}

// convertToTextRequest 转换为文本请求
func convertToTextRequest(openAIReq *OpenAIRequest, serviceName string, config *Config) (*PrivateAPITextRequest, error) {
	if len(openAIReq.Messages) == 0 {
		return nil, errors.New("输入内容为空")
	}

	// 构建私有API消息
	var messages []PrivateAPIMessage
	for _, message := range openAIReq.Messages {
		// 处理消息内容
		var contentItems []PrivateAPIContentItem

		switch content := message.Content.(type) {
		case string:
			// 如果是字符串，创建一个文本类型的内容项
			contentItems = append(contentItems, PrivateAPIContentItem{
				Type: "text",
				Text: content,
			})
		case []interface{}:
			// 如果是数组，处理每个内容项
			for _, item := range content {
				if contentMap, ok := item.(map[string]interface{}); ok {
					contentItem := PrivateAPIContentItem{
						Type: contentMap["type"].(string),
					}
					if text, ok := contentMap["text"].(string); ok {
						contentItem.Text = text
					}
					if imageURL, ok := contentMap["image_url"].(map[string]interface{}); ok {
						if url, ok := imageURL["url"].(string); ok {
							contentItem.ImageURL = &PrivateAPIImageURL{URL: url}
						}
					}
					contentItems = append(contentItems, contentItem)
				}
			}
		default:
			return nil, errors.New("不支持的消息内容格式")
		}

		privateMessage := PrivateAPIMessage{
			Role:    message.Role,
			Content: contentItems,
		}
		messages = append(messages, privateMessage)
	}

	return &PrivateAPITextRequest{
		AgentID:     config.DefaultAgentID,
		SecretKey:   config.DefaultSecretKey,
		ServiceName: serviceName,
		Messages:    messages,
		Stream:      openAIReq.Stream,
	}, nil
}

// convertToMultiModalRequest 转换为多模态请求
func convertToMultiModalRequest(openAIReq *OpenAIRequest, serviceName string, config *Config) (*PrivateAPIMultiModalRequest, error) {
	if len(openAIReq.Messages) == 0 {
		return nil, errors.New("输入内容为空")
	}

	// 构建私有API消息
	var messages []PrivateAPIMessage
	for _, message := range openAIReq.Messages {
		// 处理消息内容
		var contentItems []PrivateAPIContentItem

		switch content := message.Content.(type) {
		case string:
			// 如果是字符串，创建一个文本类型的内容项
			contentItems = append(contentItems, PrivateAPIContentItem{
				Type: "text",
				Text: content,
			})
		case []interface{}:
			// 如果是数组，处理每个内容项
			for _, item := range content {
				if contentMap, ok := item.(map[string]interface{}); ok {
					contentItem := PrivateAPIContentItem{
						Type: contentMap["type"].(string),
					}
					if text, ok := contentMap["text"].(string); ok {
						contentItem.Text = text
					}
					if imageURL, ok := contentMap["image_url"].(map[string]interface{}); ok {
						if url, ok := imageURL["url"].(string); ok {
							contentItem.ImageURL = &PrivateAPIImageURL{URL: url}
						}
					}
					contentItems = append(contentItems, contentItem)
				}
			}
		default:
			return nil, errors.New("不支持的消息内容格式")
		}

		privateMessage := PrivateAPIMessage{
			Role:    message.Role,
			Content: contentItems,
		}
		messages = append(messages, privateMessage)
	}

	return &PrivateAPIMultiModalRequest{
		AgentID:     config.DefaultAgentID,
		SecretKey:   config.DefaultSecretKey,
		ServiceName: serviceName,
		Stream:      openAIReq.Stream,
		Messages:    messages,
	}, nil
}
