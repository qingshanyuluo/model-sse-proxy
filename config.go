package main

import (
	"encoding/json"
	"os"
	"sync"
)

// Config 存储应用程序配置
type Config struct {
	// 目标API的基础URL
	TargetBaseURL string `json:"target_base_url"`
	// 默认的AgentID
	DefaultAgentID string `json:"default_agent_id"`
	// 默认的SecretKey
	DefaultSecretKey string `json:"default_secret_key"`

	// 服务器监听地址
	ServerAddress string `json:"server_address"`

	// 模型名称映射，用于将OpenAI模型名称映射到私有API的服务名称
	ModelMap map[string]string `json:"model_map,omitempty"`

	// 日志文件存储目录
	LogDirectory string `json:"log_directory"`
}

var (
	// 全局配置实例
	globalConfig *Config
	// 确保配置只被加载一次的互斥锁
	configMutex sync.Once
)

// LoadConfig 从文件加载配置
func LoadConfig(filePath string) (*Config, error) {
	var err error
	configMutex.Do(func() {
		// 默认配置
		globalConfig = &Config{
			TargetBaseURL:    "https://aibrain-large-model.hellobike.cn/AIBrainLmp/api/v1/runLargeModelApplication/run",
			DefaultAgentID:   "",
			DefaultSecretKey: "",

			ServerAddress: ":8080",
		}

		// 尝试从文件加载配置
		file, err := os.Open(filePath)
		if err != nil {
			// 如果文件不存在，使用默认配置并创建配置文件
			if os.IsNotExist(err) {
				data, _ := json.MarshalIndent(globalConfig, "", "  ")
				os.WriteFile(filePath, data, 0644)
				return
			}
			return
		}
		defer file.Close()

		// 解析JSON配置
		decoder := json.NewDecoder(file)
		err = decoder.Decode(globalConfig)
		if err != nil {
			return
		}
	})

	return globalConfig, err
}

// GetConfig 返回全局配置实例
func GetConfig() *Config {
	if globalConfig == nil {
		// 如果配置尚未加载，尝试加载默认配置文件
		LoadConfig("config.json")
	}
	return globalConfig
}
