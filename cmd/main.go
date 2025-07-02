package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	mqttclient "github.com/LanSilence/hamqtt/pkg/mqtt"
)

// CustomSensor 自定义传感器实现
type CustomSensor struct {
	currentValue float64
}

func (s *CustomSensor) GetReportData() (interface{}, error) {
	return map[string]interface{}{
		"custom_value": s.currentValue,
	}, nil
}

func (s *CustomSensor) GetSensorEntities() []mqttclient.SensorEntity {
	return []mqttclient.SensorEntity{
		{
			Name:              "custom_sensor",
			Description:       "Custom Sensor Example",
			DeviceClass:       "temperature",
			UnitOfMeasurement: "°C",
			ValueTemplate:     "custom_value",
		},
	}
}

func main() {
	// 解析命令行参数
	server := flag.String("server", "tcp://localhost", "MQTT broker address")
	port := flag.String("port", "1883", "MQTT broker port")
	user := flag.String("user", "haos", "MQTT username")
	pass := flag.String("pass", "123456", "MQTT password")
	clientID := flag.String("client-id", "hamqtt-client", "MQTT client ID")
	flag.Parse()

	// 配置MQTT客户端
	cfg := mqttclient.MQTTConfig{
		Server:   *server,
		Port:     *port,
		User:     *user,
		Pass:     *pass,
		ClientID: *clientID,
	}

	// 创建自定义传感器实例
	customSensor := &CustomSensor{currentValue: 25.0}

	// 创建MQTT客户端
	client, err := mqttclient.NewMQTTClient(cfg)
	if err != nil {
		fmt.Printf("Failed to create MQTT client: %v\n", err)
		os.Exit(1)
	}
	defer client.Stop()

	// 使用RegisterSensor注册自定义传感器
	client.RegisterSensor(
		mqttclient.SensorEntity{
			Name:              "custom_sensor",
			Description:       "Custom Sensor Example",
			DeviceClass:       "temperature",
			UnitOfMeasurement: "°C",
			ValueTemplate:     "custom_value",
		},
		nil, // 无命令处理
		func() interface{} {
			// 模拟传感器值变化
			customSensor.currentValue += 0.5
			if customSensor.currentValue > 30.0 {
				customSensor.currentValue = 25.0
			}
			return customSensor.currentValue
		},
	)

	fmt.Println("MQTT client started with custom sensor")

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down...")
}
