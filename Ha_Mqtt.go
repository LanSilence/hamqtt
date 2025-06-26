package hamqtt

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"os/exec"

	"github.com/denisbrodbeck/machineid"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

var unique_id int = 0

type SystemInfo struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemUsage    float64 `json:"mem_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	PowerStatus string  `json:"power_status"`
	Temperature float64 `json:"temperature"`
}

// type MQTTConfig struct {
// 	Server   string
// 	Port     string
// 	User     string
// 	Pass     string
// 	ClientID string
// }

type MQTTConfig struct {
	Server   string `json:"server"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Pass     string `json:"pass"`
	ClientID string `json:"client_id"`
}

type MQTTClient struct {
	client          mqtt.Client
	deviceName      string
	deviceID        string
	publishStopChan chan struct{}
}

var internalHandlers *map[string]mqtt.MessageHandler // 内部主题-回调映射
// 设置订阅主题及回调（仅保存，需重建 client 后生效）
func MqttSetTopicHandlers(topicHandlers map[string]mqtt.MessageHandler) {
	if internalHandlers == nil {
		internalHandlers = &map[string]mqtt.MessageHandler{}
	}
	for k, v := range topicHandlers {
		(*internalHandlers)[k] = v
	}
}

func handlePowerMessage(client mqtt.Client, msg mqtt.Message) {
	if !client.IsConnected() {
		return
	}
	if string(msg.Payload()) == "OFF" {
		fmt.Println("收到关机指令，准备休眠...")
		go func() {
			err := crossPlatformSuspend()
			if err != nil {
				fmt.Println("休眠失败:", err)
			} else {
				fmt.Println("休眠成功")
			}
		}()
	}
}

func getUniqueId() int {
	unique_id++
	return unique_id
}

var deviceName string = "unknown"
var deviceID string = "0000"

func initDevInfo() {
	deviceName, _ = os.Hostname()
	deviceID, _ = machineid.ID()
	deviceID = deviceID[:4]
}

/*
	{
	    "name": "Device Power",
	    "command_topic": "homeassistant/switch/ubuntu62c4/set",
	    "state_topic": "homeassistant/sensor/ubuntu62c4/state",
	    "unique_id": "62c403",
	    "device_class": "switch",
	                "value_template":"{{ value_json.power_status }}",
	    "payload_on": "ON",
	    "payload_off": "OFF",
	    "device": {
	        "identifiers": ["ubuntu62c4"],
	        "name": "ubuntu",
	        "manufacturer": "Custom Device"
	    }
	}
*/
func getPayload(name string, device_class string, unit_of_measurement string, value_template string) string {
	payloadStr := ""
	if device_class == "switch" {
		payloadStr = `
		{
  			"name": "` + name + `",
			"command_topic": "homeassistant/switch/` + deviceName + deviceID + `/set",
  			"state_topic": "homeassistant/sensor/` + deviceName + deviceID + `/state",
			"unique_id":"` + deviceID + fmt.Sprintf("%d", getUniqueId()) + `",
			"device_class": "` + device_class + `",
  			"value_template": "{{ ` + value_template + ` }}",
			"payload_on": "ON",
			"payload_off": "OFF",
  			"device": {
  			  "identifiers":["` + deviceName + deviceID + `"],
  			  "name": "` + deviceName + `",
  			  "manufacturer": "HaPerfMonitor Device"
  			}
		}`

	} else {
		payloadStr = `
    	{
  			"name": "` + name + `",
			"device_class": "` + device_class + `",
  			"state_topic": "homeassistant/sensor/` + deviceName + deviceID + `/state",
  			"unit_of_measurement": "` + unit_of_measurement + `",
			"unique_id":"` + deviceID + fmt.Sprintf("%d", getUniqueId()) + `",
  			"value_template": "{{ ` + value_template + ` }}",
  			"device": {
  			  "identifiers":["` + deviceName + deviceID + `"],
  			  "name": "` + deviceName + `",
  			  "manufacturer": "HaPerfMonitor Device"
  			}
		}`
	}

	return payloadStr
}

func getTopic(device_class string, sensorType string) string {
	return "homeassistant/" + device_class + "/" + deviceName + deviceID + sensorType + "/config"
}

func NewMQTTClient(cfg MQTTConfig) (*MQTTClient, error) {
	initDevInfo()
	client := &MQTTClient{
		deviceName: deviceName,
		deviceID:   deviceID,
	}
	broker := cfg.Server + ":" + cfg.Port
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.User)
	opts.SetPassword(cfg.Pass)
	// 设置LWT，掉线时自动推送OFF
	opts.SetWill("homeassistant/sensor/"+deviceName+deviceID+"/state", "OFF", 1, true)
	// 注册自动订阅
	if internalHandlers == nil {
		internalHandlers = &map[string]mqtt.MessageHandler{"homeassistant/switch/" + deviceName + deviceID + "/set": handlePowerMessage}

	}
	if internalHandlers != nil {
		setOnConnectSubscribe(opts)
	}
	client.client = mqtt.NewClient(opts)
	if token := client.client.Connect(); token.WaitTimeout(time.Second*5) && token.Error() != nil {
		return nil, token.Error()
	}
	// 发布配置主题
	mqttTopicMem := getTopic("sensor", "mem")
	mqttTopicCpu := getTopic("sensor", "cpu")
	mqttTopicSwitch := getTopic("switch", "power")
	mqttTopicTemp := getTopic("sensor", "temperature")

	payloadMem := getPayload("Memory Usage", "humidity", "%", "value_json.mem_usage")
	payloadCpu := getPayload("CPU Usage", "humidity", "%", "value_json.cpu_usage")
	payloadSwitch := getPayload("Device Power", "switch", "", "value_json.power_status")
	payloadTemp := getPayload("Device Temperature", "temperature", "°C", "value_json.temperature")

	token := client.client.Publish(mqttTopicMem, 1, true, payloadMem)

	client.client.Publish(mqttTopicCpu, 1, true, payloadCpu)

	client.client.Publish(mqttTopicSwitch, 1, true, payloadSwitch)
	client.client.Publish(mqttTopicTemp, 1, true, payloadTemp)
	token.Wait()
	client.publishStopChan = make(chan struct{})
	// 订阅set主题，收到OFF时休眠

	go client.publishServerStatus()
	return client, nil
}

// 新的订阅注册方式，需在创建 client 前设置 OnConnect
func setOnConnectSubscribe(opts *mqtt.ClientOptions) {
	oldHandler := opts.OnConnect
	opts.OnConnect = func(c mqtt.Client) {
		if oldHandler != nil {
			oldHandler(c)
		}
		for topic, handler := range *internalHandlers {
			token := c.Subscribe(topic, 1, handler)
			token.Wait()
			if token.Error() != nil {
				fmt.Println("订阅失败:", topic, token.Error())
			} else {
				fmt.Println("已订阅主题:", topic)
			}
		}
	}
}

func (c *MQTTClient) publishServerStatus() {
	retryCount := 0
	retryInterval := 5 * time.Second
	stop := c.publishStopChan
	for {
		select {
		case <-stop:
			fmt.Println("publishServerStatus goroutine exit by stop signal")
			return
		default:
		}
		if c.client == nil || !c.client.IsConnected() {
			for {
				select {
				case <-stop:
					fmt.Println("publishServerStatus goroutine exit by stop signal")
					return
				default:
				}
				fmt.Println("MQTT断开，尝试重连...")
				if token := c.client.Connect(); token.WaitTimeout(5*time.Second) && token.Error() == nil {
					fmt.Println("MQTT重连成功")
					retryCount = 0
					retryInterval = 5 * time.Second
					break
				} else {
					retryCount++
					retryInterval = time.Duration(5*retryCount) * time.Second
					if retryInterval > 60*time.Second {
						retryInterval = 60 * time.Second
					}
					fmt.Printf("重连失败，%d秒后重试...\n", int(retryInterval.Seconds()))
					time.Sleep(retryInterval)
				}
			}
		}
		var cpuSum float64
		samples := 2
		for i := 0; i < samples; i++ {
			percentages, err := cpu.Percent(time.Millisecond*500, false)
			if err == nil && len(percentages) > 0 {
				cpuSum += percentages[0]
			}
		}
		cpuAvg := cpuSum / float64(samples)
		// 仅在 Windows 下修正 cpuAvg
		if getOSType() == "windows" && cpuAvg < 10 {
			cpuAvg = cpuAvg * 10
		}
		mem, _ := mem.VirtualMemory()
		memPercent := float64(mem.Used) / float64(mem.Total) * 100
		disks, _ := disk.Usage("/")
		diskPercent := float64(disks.Used) / float64(disks.Total) * 100
		PowerStatus := "ON"
		temperature := getDeviceTemperature()
		info := SystemInfo{
			CPUUsage:    cpuAvg,
			MemUsage:    memPercent,
			DiskUsage:   diskPercent,
			PowerStatus: PowerStatus, // 新增字段
			Temperature: temperature, // 新增字段
		}
		stateTopic := "homeassistant/sensor/" + c.deviceName + c.deviceID + "/state"
		payload, err := json.Marshal(info)
		if err != nil {
			panic(err)
		}
		token := c.client.Publish(stateTopic, 0, false, payload)
		token.Wait()
		time.Sleep(2 * time.Second)
	}
}

func (c *MQTTClient) Stop() {
	if c.publishStopChan != nil {
		close(c.publishStopChan)
		c.publishStopChan = nil
	}
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}

// 跨平台休眠辅助函数
func crossPlatformSuspend() error {
	osType := getOSType()
	switch osType {
	case "windows":
		return exec.Command("cmd", "/C", "rundll32.exe powrprof.dll,SetSuspendState 0,1,0").Run()
	case "linux":
		return exec.Command("systemctl", "suspend").Run()
	case "darwin":
		return exec.Command("pmset", "poweroff").Run()
	default:
		return fmt.Errorf("不支持的操作系统: %s", osType)
	}
}

func getOSType() string {
	return runtime.GOOS
}
