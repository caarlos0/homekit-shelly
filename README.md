# homekit-shelly

This project adds Homekit functionality to the Shelly Flood, Shelly Smoke and
Shelly Plus H&T.

You need to a MQQT service, and set both this project and your Shellies
to use it.

You can then run this project with:

```bash
HT=Device1ID,Device2ID SMOKES=Device1ID,Device2ID FLOODS=Device1ID,Device2ID,etc homekit-shelly
```

Then, add the Shelly Bridge to your Home, and everything should work.

The PIN is `001-02-003`.

When an event happens in the Shellies, they will send it to MQTT, which
homekit-shelly will be listening to, and will act upon.

Previous responses are cached so it works well across restarts.
