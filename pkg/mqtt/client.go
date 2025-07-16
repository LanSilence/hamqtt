package mqtt

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"os/exec"

	"github.com/LanSilence/hamqtt/internal/system"
	"github.com/LanSilence/hamqtt/pkg"
	"github.com/denisbrodbeck/machineid"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

var unique_id int = 0

// SensorEntity 定义传感器实体
type SensorEntity struct {
	Name              string
	Description       string
	DeviceClass       string
	UnitOfMeasurement string
	ValueTemplate     string
}

type SystemInfo struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemUsage    float64 `json:"mem_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	PowerStatus string  `json:"power_status"`
	Temperature float64 `json:"temperature"`
	// 自定义字段将直接合并到顶层
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
	sensors         []SensorEntity // 直接注册的传感器
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

func initDevInfo(cfg MQTTConfig) {
	deviceName, _ = os.Hostname()
	deviceID, _ = machineid.ID()
	deviceID = cfg.ClientID
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
	uniqueID := deviceID + "_" + name
	payload := map[string]interface{}{
		"name":           name,
		"device_class":   device_class,
		"state_topic":    "homeassistant/sensor/" + deviceName + deviceID + "/state",
		"unique_id":      uniqueID,
		"value_template": "{{ " + value_template + " }}",
		"device": map[string]interface{}{
			"identifiers":  []string{deviceName + deviceID},
			"name":         deviceName,
			"manufacturer": "HaPerfMonitor",
			"model":        "MQTT Monitor",
			"sw_version":   "1.0",
		},
	}

	if device_class == "switch" {
		payload["command_topic"] = "homeassistant/switch/" + deviceName + deviceID + "/set"
		payload["payload_on"] = "ON"
		payload["payload_off"] = "OFF"
	} else if unit_of_measurement != "" {
		payload["unit_of_measurement"] = unit_of_measurement
	}

	jsonData, _ := json.MarshalIndent(payload, "", "  ")
	return string(jsonData)
}

func getTopic(deviceClass string, sensorName string) string {
	// 根据HomeAssistant MQTT自动发现规范构建主题
	// 主题格式: homeassistant/<component>/[<node_id>/]<object_id>/config
	component := "sensor" // 默认为传感器
	switch deviceClass {
	case "switch":
		component = "switch"
	case "binary_sensor":
		component = "binary_sensor"
	case "light":
		component = "light"
	}
	return "homeassistant/" + component + "/" + deviceName + deviceID + "/" + sensorName + "/config"
}

func NewMQTTClient(cfg MQTTConfig) (*MQTTClient, error) {
	initDevInfo(cfg)
	client := &MQTTClient{
		deviceName: deviceName,
		deviceID:   cfg.ClientID,
	}

	// 注册默认实体
	defaultEntities := []SensorEntity{
		{
			Name:              "memory",
			Description:       "Memory Usage",
			DeviceClass:       "humidity",
			UnitOfMeasurement: "%",
			ValueTemplate:     "value_json.mem_usage",
		},
		{
			Name:              "cpu",
			Description:       "CPU Usage",
			DeviceClass:       "humidity",
			UnitOfMeasurement: "%",
			ValueTemplate:     "value_json.cpu_usage",
		},
		{
			Name:              "power",
			Description:       "Device Power",
			DeviceClass:       "switch",
			UnitOfMeasurement: "",
			ValueTemplate:     "value_json.power_status",
		},
		{
			Name:              "temperature",
			Description:       "Device Temperature",
			DeviceClass:       "temperature",
			UnitOfMeasurement: "°C",
			ValueTemplate:     "value_json.temperature",
		},
	}

	broker := cfg.Server + ":" + cfg.Port
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.User)
	opts.SetPassword(cfg.Pass)
	// 设置LWT和可用性主题
	opts.SetWill("homeassistant/sensor/"+deviceName+deviceID+"/status", "OFF", 1, false)
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
	// 发布在线状态
	client.client.Publish("homeassistant/sensor/"+deviceName+deviceID+"/status", 1, true, "online")

	// 注册自定义实体

	// 发布默认实体配置
	for _, entity := range defaultEntities {
		topic := getTopic(entity.DeviceClass, entity.Name)
		payload := getPayload(entity.Name, entity.DeviceClass,
			entity.UnitOfMeasurement, entity.ValueTemplate)
		token := client.client.Publish(topic, 1, true, payload)
		token.Wait()
	}
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
		if pkg.GetOSType() == "windows" && cpuAvg < 10 {
			cpuAvg = cpuAvg * 10
		}
		mem, _ := mem.VirtualMemory()
		memPercent := float64(mem.Used) / float64(mem.Total) * 100
		disks, _ := disk.Usage("/")
		diskPercent := float64(disks.Used) / float64(disks.Total) * 100
		PowerStatus := "ON"
		temperature := system.GetDeviceTemperature()
		// 创建基础状态信息
		info := map[string]interface{}{
			"cpu_usage":    cpuAvg,
			"mem_usage":    memPercent,
			"disk_usage":   diskPercent,
			"power_status": PowerStatus,
			"temperature":  temperature,
		}

		stateTopic := "homeassistant/sensor/" + c.deviceName + c.deviceID + "/state"
		payload, err := json.Marshal(info)
		if err != nil {
			panic(err)
		}
		token := c.client.Publish(stateTopic, 1, true, payload)
		token.Wait()
		time.Sleep(2 * time.Second)
	}
}

// RegisterSensor 直接注册一个传感器实体
// commandHandler - 处理命令消息的可选回调
// stateHandler - 返回当前状态值的可选回调
func (c *MQTTClient) RegisterSensor(entity SensorEntity,
	commandHandler mqtt.MessageHandler,
	stateHandler func() interface{}) {

	c.sensors = append(c.sensors, entity)
	topic := getTopic(entity.DeviceClass, entity.Name)
	payload := getPayload(entity.Name, entity.DeviceClass,
		entity.UnitOfMeasurement, entity.ValueTemplate)
	if c.client != nil && c.client.IsConnected() {
		c.client.Publish(topic, 1, true, payload)

		// 注册命令处理handler
		if commandHandler != nil {
			MqttSetTopicHandlers(map[string]mqtt.MessageHandler{
				"homeassistant/" + entity.DeviceClass + "/" + c.deviceName + c.deviceID + "/" + entity.Name + "/set": commandHandler,
			})
			if c.client.IsConnected() {
				token := c.client.Subscribe(
					"homeassistant/"+entity.DeviceClass+"/"+c.deviceName+c.deviceID+"/"+entity.Name+"/set",
					1,
					commandHandler,
				)
				token.Wait()
			}
		}

		// 注册状态更新处理
		if stateHandler != nil {
			go func() {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						state := stateHandler()
						payload, _ := json.Marshal(state)
						c.client.Publish(
							"homeassistant/sensor/"+c.deviceName+c.deviceID+"/state",
							1,
							true,
							payload,
						)
					case <-c.publishStopChan:
						return
					}
				}
			}()
		}
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
	osType := pkg.GetOSType()
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
