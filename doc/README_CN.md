# HaMQTT

HomeAssistant MQTT 客户端，用于系统监控和控制

## 功能特性

- 发布系统指标(CPU、内存、磁盘使用率)到MQTT
- 支持HomeAssistant MQTT自动发现
- 自定义传感器注册
- 电源管理(睡眠/关机)

## 安装

### 作为库使用
```bash
# 获取最新版本
go get github.com/LanSilence/hamqtt

# 获取指定版本(将v1.2.3替换为实际版本号)
go get github.com/LanSilence/hamqtt@v1.2.3

# 也可以在go.mod中指定版本:
require github.com/LanSilence/hamqtt v1.2.3
```

### 作为独立工具
```bash
go build -o hamqtt ./cmd/main.go
```

## 使用

```bash
./hamqtt --server tcp://mqtt-broker --port 1883
```

### 命令行选项

```
--server      MQTT代理地址 (默认 "tcp://localhost")
--port        MQTT代理端口 (默认 "1883")
--user        MQTT用户名
--pass        MQTT密码
--client-id   MQTT客户端ID (默认 "hamqtt-client")
```

## 库使用方式

作为库导入和使用:

```go
import "github.com/LanSilence/hamqtt/pkg/mqtt"

func main() {
    cfg := mqtt.MQTTConfig{
        Server:   "tcp://localhost",
        Port:     "1883",
        User:     "user",
        Pass:     "pass",
        ClientID: "my-client",
    }

    client, err := mqtt.NewMQTTClient(cfg)
    if err != nil {
        panic(err)
    }
    defer client.Stop()

    // 注册传感器或使用其他功能...
}

## 自定义传感器

直接注册传感器并设置状态更新回调:

```go
client.RegisterSensor(
    mqttclient.SensorEntity{
        Name: "custom",
        Description: "自定义传感器",
        DeviceClass: "temperature",
        UnitOfMeasurement: "°C",
        ValueTemplate: "value",
    },
    nil, // 无命令处理
    func() interface{} {
        return 42 // 返回当前传感器值
    },
)
```

[查看英文文档](../README.md)
