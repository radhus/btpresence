package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

var (
	mqttLogger = log.New(os.Stdout, "[mqtt] ", 0)
)

func init() {
	mqtt.ERROR = mqttLogger
	mqtt.CRITICAL = mqttLogger
	mqtt.WARN = mqttLogger
}

func connectMQTT(url string) (mqtt.Client, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	clientID := fmt.Sprintf("btpresence-%s-%d", hostname, time.Now().Unix())
	opts := mqtt.NewClientOptions().AddBroker(url).SetClientID(clientID)
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); !token.Wait() || token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT: %w", token.Error())
	}

	return client, nil
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln("Hostname failed:", err)
	}
	defaultTopic := fmt.Sprintf("btpresence/%s", hostname)

	mqttURL := flag.String("url", "", "MQTT URL")
	mqttTopicPrefix := flag.String("prefix", defaultTopic, "MQTT topic prefix")
	flag.Parse()
	if *mqttURL == "" {
		flag.Usage()
		log.Fatalln("Need to specify MQTT URL")
	}

	prefix := *mqttTopicPrefix
	for strings.HasSuffix(prefix, "/") {
		prefix = strings.TrimSuffix(prefix, "/")
	}

	defer api.Exit()

	a, err := adapter.GetAdapter("hci0")
	if err != nil {
		log.Fatalln("GetAdapter failed:", err)
	}

	log.Println("Flushing device cache")
	err = a.FlushDevices()
	if err != nil {
		log.Fatalln("FlushDevices failed:", err)
	}

	mqttConn, err := connectMQTT(*mqttURL)
	if err != nil {
		log.Fatalln("Failed to setup MQTT:", err)
	}

	log.Println("Start discovery")
	discovery, cancel, err := api.Discover(a, nil)
	if err != nil {
		log.Fatalln("Discover failed:", err)
	}
	defer func() { cancel() }()

	ticker := time.NewTicker(time.Minute)

	for {
		select {
		case <-ticker.C:
			log.Println("Canceling discovery")
			cancel()

			log.Println("Flushing device cache")
			err = a.FlushDevices()
			if err != nil {
				log.Fatalln("FlushDevices failed:", err)
			}

			log.Println("Start discovery")
			discovery, cancel, err = api.Discover(a, nil)
			if err != nil {
				log.Fatalln("Discover failed:", err)
			}

		case ev := <-discovery:
			if ev.Type == adapter.DeviceRemoved {
				log.Println("Device removed:", ev)
			}

			dev, err := device.NewDevice1(ev.Path)
			if err != nil {
				log.Println("NewDevice1 failed:", ev.Path, err)
				continue
			}

			if dev == nil {
				log.Println("NewDevice1 failed, dev == nil:", ev.Path)
				continue
			}

			publish := func(id, data string) {
				topic := path.Join(prefix, dev.Properties.Address, id)
				token := mqttConn.Publish(topic, 0, true, data)
				token.Wait()
				if err := token.Error(); err != nil {
					log.Fatalln("Failed to publish:", err)
				}
			}

			publish("seen", fmt.Sprintf("%d", time.Now().Unix()))
			publish("rssi", fmt.Sprintf("%d", dev.Properties.RSSI))
			publish("name", fmt.Sprintf("%s", dev.Properties.Name))
		}
	}
}
