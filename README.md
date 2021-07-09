# btpresence

Publishes MQTT message of found Bluetooth devices.

Only works on Linux with D-Bus BlueZ.

## Usage

```bash
btpresence -url mqtt://mqtt.lan:1883
```

Will publish topics like:
```
btpresence/hostname/AA:BB:CC:DD:EE:FF/seen  1625839741
btpresence/hostname/AA:BB:CC:DD:EE:FF/rssi  -69
btpresence/hostname/AA:BB:CC:DD:EE:FF/name  Device
```
