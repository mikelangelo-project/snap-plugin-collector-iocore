# snap collector plugin - iocore

This plugin gather iocore statistics from /sys/class/vhost/
															
The plugin is used in the [snap framework] (http://github.com/intelsdi-x/snap).				

1. [Getting Started](#getting-started)
  * [System Requirements](#system-requirements)
  * [Installation](#installation)
  * [Configuration and Usage](#configuration-and-usage)
2. [Documentation](#documentation)
  * [Collected Metrics](#collected-metrics)
  * [Examples](#examples)

## Getting Started

### System Requirements

- Linux system

### Installation

#### To build the plugin binary:

Fork https://gitlab.xlab.si/yossi_kuperman/snap-plugin-collector-iocore
Clone repo into `$GOPATH/src/github.com/intelsdi-x/`:

```
$ git clone https://gitlab.xlab.si/yossi_kuperman/snap-plugin-collector-iocore.git
```

Build the snap iocore plugin by running make within the cloned repo:
```
$ make
```
This builds the plugin in `/build/`

### Configuration and Usage

* Set up the [snap framework](https://github.com/intelsdi-x/snap/blob/master/README.md#getting-started)
* Load the plugin and create a task, see example in examples directory.

Configuration parameters:
- `vhost_path` path to 'vhost' directory

## Documentation

This plugin has the ability to read metrics from IOcm-enabled kernel. The exposed metrics are the number of active I/O cores and their respective effective utilization.

### Collected Metrics
This plugin has the ability to gather the following metrics:
                                                                                                
Metric namespace is `/ibm/sysfs/iocore/<iocore_name>/cpu_utilization` where `<iocore_device>` expands to cpu (aggregated), cpu1, cpu2, cpu3 and so on.
This is metric is the effective percentage utilized by an iocore. 

Additionally, the number of active I/O cores is available here: `/ibm/sysfs/iocore/nr_iocores`

Data type of all above metrics is float64.

By default metrics are gathered once per second.

### Examples

Example of running snap iocore collector and writing data to file.

Run the snap daemon:
```
$ snapd -l 1 -t 0
```

Load iocore plugin for collecting:
```
$ snapctl plugin load $SNAP_IOCORE_PLUGIN_DIR/build/linux/x86_64/snap-plugin-collector-iocore
```
See all available metrics:
```
$ snapctl metric list
```

Get file plugin for publishing, appropriate for Linux or Darwin:
```
$ wget  http://snap.ci.snap-telemetry.io/plugins/snap-plugin-publisher-file/latest/linux/x86_64/snap-plugin-publisher-file
```
or
```
$ wget  http://snap.ci.snap-telemetry.io/plugins/snap-plugin-publisher-file/latest/darwin/x86_64/snap-plugin-publisher-file
```

Load file plugin for publishing:
```
$ snapctl plugin load snap-plugin-publisher-file
```

Create a task JSON file (exemplary file in examples):
```json
{
    "version": 1,
    "schedule": {
        "type": "simple",
        "interval": "1s"
    },
    "workflow": {
        "collect": {
            "metrics": {
                "/ibm/sysfs/iocore/*/cpu_utilization": {},
                "/ibm/sysfs/iocore/nr_iocores": {}
            },
            "process": [
                {
                    "plugin_name": "passthru",
                    "process": null,
                    "publish": [
                        {
                            "plugin_name": "file",
                            "config": {
                                "file": "/tmp/published_load"
                            }
                        }
                    ],
                    "config": null
                }
            ],
            "publish": null
        }
    }
}
```

Create a task:
```
$ snapctl task create -t $SNAP_IOCORE_PLUGIN_DIR/example/iocore-file.json
Using task manifest to create task
Task created
ID: 480323af-15b0-4af8-a526-eb2ca6d8ae67
Name: Task-480323af-15b0-4af8-a526-eb2ca6d8ae67
State: Running
```

Stop task:
```
$ snapctl task stop 480323af-15b0-4af8-a526-eb2ca6d8ae67
Task stopped:
ID: 480323af-15b0-4af8-a526-eb2ca6d8ae67
```

## Acknowledgements

This project has been conducted within the RIA [MIKELANGELO 
project](https://www.mikelangelo-project.eu) (no.  645402), started in January
2015, and co-funded by the European Commission under the H2020-ICT- 07-2014:
Advanced Cloud Infrastructures and Services programme.
