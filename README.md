# Model SSE Proxy

这是一个代理服务器，用于将OpenAI格式的API请求转换为私有协议格式，并支持SSE（Server-Sent Events）流式响应。

## 功能特点

- 支持OpenAI格式的API请求转换为私有协议格式
- 支持文本和多模态（图片）请求
- 支持SSE流式响应
- 支持外部配置文件
- 支持CORS跨域请求

## 配置说明

配置文件为`config.json`，包含以下字段：

```json
{
  "target_base_url": "https://example.com/api/endpoint",  // 目标API的基础URL
  "default_agent_id": "your_agent_id",                 // 默认的AgentID
  "default_secret_key": "your_secret_key",             // 默认的SecretKey
  "model_mapping": {                                    // 模型映射，从OpenAI模型名称映射到目标服务的模型名称
    "gpt-4": "Qwen-72B",
    "gpt-4-vision": "Qwen-VL-Plus",
    "gpt-4.1": "Qwen-72B"
  },
  "server_address": ":8080"                            // 服务器监听地址
}
```

## 使用方法

### 启动服务

```bash
go run .
```

或者编译后运行：

```bash
go build
./model-sse-proxy
```

### 发送请求

#### 文本请求示例

```bash
curl "http://localhost:8080/v1/responses" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer your_api_key" \
    -d '{ 
        "model": "gpt-4.1", 
        "input": [ 
            { 
                "role": "user", 
                "content": "介绍一下世界杯" 
            } 
        ] 
    }'
```

#### 图片请求示例

```bash
curl "http://localhost:8080/v1/responses" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer your_api_key" \
    -d '{ 
        "model": "gpt-4-vision", 
        "input": [ 
            { 
                "role": "user", 
                "content": "What two teams are playing in this photo?" 
            }, 
            { 
                "role": "user", 
                "content": [ 
                    { 
                        "type": "input_image", 
                        "image_url": "https://example.com/image.jpg" 
                    } 
                ] 
            } 
        ] 
    }'
```

## 注意事项

- 请确保在`config.json`中正确配置目标API的URL、AgentID和SecretKey
- 请确保模型映射正确，以便正确转换请求
- 服务器默认监听在`:8080`端口，可以在配置文件中修改