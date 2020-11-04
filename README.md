# Ovanice

Ovanice is a small service that hosts your Open Virtual Appliance-files. Ovanice gives you the option to provide configuration options on request and will rewrite the .ova-file on the fly.

## Download and usage

- Grab the latest binary from the [releases](https://github.com/felixandersen/ovanice/releases) page and run it in a directory containing ova-files:

```shell
./ovanice
```

- An inventory in JSON format is available at the `/inventory`-endpoint. Here piped to jq for clarity.

```shell
curl -s http://localhost:8080/inventory | jq
{
  "ova_files": [
    {
      "name": "minimal_test.ova",
      "virtual_hardware_items": [
        {
          "ElementName": "16 virtual CPU(s)",
          "Description": "Number of Virtual CPUs",
          "ResourceType": 3,
          "VirtualQuantity": 16,
          "param": "cores"
        },
        {
          "ElementName": "16384MB of memory",
          "Description": "Memory Size",
          "ResourceType": 4,
          "VirtualQuantity": 16,
          "param": "ram"
        }
      ],
      "example_request": "/ova_files/minimal_test.ova?cores=4&ram=2048"
    }
  ]
}
```

- Each item in the inventory provides an example request. A GET request to `http://localhost:8080/ova_files/minimal_test.ova?cores=4&ram=2048` will return the `minimal_test.ova` appliance but rewritten to instead have 4 cores and 2GB RAM.


## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.
