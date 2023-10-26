# homekit-shelly

This project adds Homekit functionality to the Shelly Flood and Shelly Smoke.

You need to a MQQT service (I tested with Mosquitto), and set both this project
and your Shelly's to use it.

You can then run this project with:

```bash
SMOKES=Device1ID,Device2ID FLOODS=Device1ID,Device2ID,etc homekit-shelly
```

Then, add the Shelly Bridge to your Home, and everything should work.

The PIN is `001-02-003`.

When an event happens in the Shelly's, they will send it to mqtt, which
homekit-shelly will be listening to.

Previous responses are cached so it works across restarts as well.
