# HaMQTT

[中文文档](doc/README_CN.md)

HomeAssistant MQTT Client for system monitoring and control

## Features

- Publish system metrics (CPU, Memory, Disk usage) to MQTT
- Support HomeAssistant MQTT auto-discovery
- Custom sensor registration
- Power management (sleep/shutdown)

## Installation

### As a library
```bash
# Get latest version
go get github.com/LanSilence/hamqtt

# Get specific version (replace v0.2.0 with actual version)
go get github.com/LanSilence/hamqtt@v0.2.0

# In your go.mod, you can also specify version:
require github.com/LanSilence/hamqtt v0.2.0
```

### As a standalone tool
```bash
go build -o hamqtt ./cmd/main.go
```

## Usage

```bash
./hamqtt --server tcp://mqtt-broker --port 1883
```

### Command Line Options

```
--server      MQTT broker address (default "tcp://localhost")
--port        MQTT broker port (default "1883")
--user        MQTT username
--pass        MQTT password
--client-id   MQTT client ID (default "hamqtt-client")
```

## Library Usage

Import and use this package as a library:

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

    // Register sensors or use other features...
}

## Custom Sensors

Directly register sensors with state update callback:

client.RegisterSensor(
    mqttclient.SensorEntity{
        Name: "custom",
        Description: "Custom Sensor",
        DeviceClass: "temperature",
        UnitOfMeasurement: "°C",
        ValueTemplate: "value",
    },
    nil, // no command handler
    func() interface{} {
        return 42 // return current sensor value
    },
)
```
