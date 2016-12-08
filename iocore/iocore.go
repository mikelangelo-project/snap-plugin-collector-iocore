/*
http://www.apache.org/licenses/LICENSE-2.0.txt
Copyright 2016 IBM Corporation
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/


//
// This snap plugin exposes two metrics:
//
// 1. /ibm/sysfs/iocore/nr_iocores
// Returns the number of active I/O cores
//
// 2. /ibm/sysfs/iocore/*/cpu_utilization
// Returns the effective CPU utilization (cpu1, cpu2, cpu (aggregated))
//


package iocore

import (
	"bufio"
	"io/ioutil"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"math"

	log "github.com/Sirupsen/logrus"

	"github.com/intelsdi-x/snap/control/plugin"
	"github.com/intelsdi-x/snap/control/plugin/cpolicy"
	"github.com/intelsdi-x/snap/core"

	"github.com/intelsdi-x/snap-plugin-utilities/config"
)

const (
	// Name of plugin
	pluginName = "iocore"
	// Version of plugin
	pluginVersion = 3
	// Type of plugin
	pluginType = plugin.CollectorPluginType

	nsVendor = "ibm"
	nsClass  = "sysfs"
	nsType   = "iocore"

	uintMax = ^uint64(0)
)

const (
	nTotalWorkCycles = "total_work_cycles"
	nTotalCycles     = "total_cycles"
	// name of exposed metrics
	nCpuUtilization  = "cpu_utilization"
	nNumIOcores      = "nr_iocores"
)

// IOCoreCollector holds iocore statistics
type IOCoreCollector struct {
	data     iocoreStats
	dataPrev iocoreStats        // previous data, to calculate derivatives
	output   map[string]float64 // contains exposed metrics and their value (calculated based on data & dataPrev)
	first    bool               // is true for first collecting (do not calculate derivatives), after that set false
}

type iocoreStats struct {
	stats     map[string]map[string]uint64
	timestamp time.Time
}

// prefix in metric namespace
var prefix = []string{nsVendor, nsClass, nsType}

// New returns snap-plugin-collector-iocore instance
func New() (*IOCoreCollector, error) {
	dc := &IOCoreCollector{data: iocoreStats{stats: map[string]map[string]uint64{}, timestamp: time.Now()},
		dataPrev: iocoreStats{stats: map[string]map[string]uint64{}, timestamp: time.Now()},
		output:   map[string]float64{},
		first:    true}
	return dc, nil
}

// Meta returns plugin meta data
func Meta() *plugin.PluginMeta {
	return plugin.NewPluginMeta(
		pluginName,
		pluginVersion,
		pluginType,
		[]string{},
		[]string{plugin.SnapGOBContentType},
		plugin.ConcurrencyCount(1),
	)
}

// GetConfigPolicy returns a ConfigPolicy
func (dc *IOCoreCollector) GetConfigPolicy() (*cpolicy.ConfigPolicy, error) {
	cp := cpolicy.New()
	rule, _ := cpolicy.NewStringRule("vhost_path", false, "/sys/class/vhost")
	node := cpolicy.NewPolicyNode()
	node.Add(rule)
	cp.Add([]string{nsVendor, nsClass, pluginName}, node)
	return cp, nil
}

// GetMetricTypes returns list of exposed I/O core stats metrics
func (dc *IOCoreCollector) GetMetricTypes(cfg plugin.ConfigType) ([]plugin.MetricType, error) {
	mts := []plugin.MetricType{}

	mts = append(mts, plugin.MetricType{
			Namespace_:   core.NewNamespace(append(prefix, []string{nNumIOcores}...)...),
			Description_: "number of active I/O cores",
			Unit_:        "",
			Config_:      cfg.ConfigDataNode,
	})

	mts = append(mts, plugin.MetricType{
		Namespace_: core.NewNamespace(prefix...).
			AddDynamicElement("iocore", "I/O core effective utilization (percentage)").
			AddStaticElement(nCpuUtilization),
		Description_: "dynamic iocore metric: " + nCpuUtilization,
	})

	return mts, nil
}

// CollectMetrics retrieves I/O cores stats values for given metrics
func (dc *IOCoreCollector) CollectMetrics(mts []plugin.MetricType) ([]plugin.MetricType, error) {
	metrics := []plugin.MetricType{}

	vhostPath, err := getVhostPath(mts[0])
	if err != nil {
		return nil, err
	}

	first := dc.first // true if collecting for the first time
	if first {
		dc.first = false
	}

	// for first collecting skip stash previous data
	if !first {
		// stash iocores data (dst, src)
		stashData(&dc.dataPrev, &dc.data)
	}

	// clear the output map
	for key := range dc.output {
   		delete(dc.output, key)
	}

	// get current cpu utilization per I/O core
	if err := dc.getIOCoreStats(vhostPath); err != nil {
		return nil, err
	}

	// get active number of I/O cores
	nr_iocores, err := dc.getNumOfIOCores(vhostPath)
	if err != nil {
		return nil, err
	}

	//  for first collecting skip derivatives calculation
	if !first {
		// calculate derivatives based on data (presence) and previous one; results stored in dc.output
		if err := dc.calcDerivatives(); err != nil {
			return nil, err
		}
	}

	dc.output[nNumIOcores] = float64(nr_iocores)

	for _, m := range mts {
		ns := m.Namespace()
		if ns[len(ns)-2].Value == "*" {
			found := false
			for i := range dc.output {
				cMetric := strings.Split(i, "/")
				if cMetric[len(cMetric)-1] == ns[len(ns)-1].Value {
					ns1 := core.NewNamespace(createNamespace(i)...)
					ns1[len(ns1)-2].Name = ns[len(ns)-2].Name
					metric := plugin.MetricType{
						Namespace_: ns1,
						Data_:      dc.output[i],
						Timestamp_: dc.data.timestamp,
					}
					metrics = append(metrics, metric)
					found = true
				}
			}
			if !found {
				for i := range dc.data.stats {
					cMetric := strings.Split(i, "/")
					if cMetric[len(cMetric)-1] == ns[len(ns)-1].Value {
						ns1 := core.NewNamespace(createNamespace(i)...)
						ns1[len(ns1)-2].Name = ns[len(ns)-2].Name
						metric := plugin.MetricType{
							Namespace_: ns1,
							// first time all values are zero
							Data_:      0,
							Timestamp_: dc.data.timestamp,
						}
						metrics = append(metrics, metric)
					}
				}
			}
		} else {
			if v, ok := dc.output[parseNamespace(m.Namespace().Strings())]; ok {
				metric := plugin.MetricType{
					Namespace_: m.Namespace(),
					Data_:      v,
					Timestamp_: dc.data.timestamp,
				}
				metrics = append(metrics, metric)
			} else {
				log.Warning(fmt.Sprintf("Can not find static metric value for %s", m.Namespace().Strings()))
			}
		}
	}

	return metrics, nil
}

// getIOCoreStats gets iocore stats from file (/sys/class/vhost/) and stores them in the IOCoreCollector structure
func (dc *IOCoreCollector) getIOCoreStats(vhostPath string) error {
	srcFile := path.Join(vhostPath, "iocores_utilization")

	fh, err := os.Open(srcFile)
	defer fh.Close()

	if err != nil {
		return fmt.Errorf("Error opening %s, error = %s", srcFile, err)
	}
	scanner := bufio.NewScanner(fh)
	dc.data.timestamp = time.Now()

	// clear the stats map
	for key := range dc.data.stats {
   		delete(dc.data.stats, key)
	}

	// scan file content
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		iocoreName := fields[0]

		//map iocore statistics keys (names) to scanned fields
		data := make(map[string]uint64)
		data[nTotalWorkCycles], _ = strconv.ParseUint(fields[1], 10, 64)
		data[nTotalCycles], _     = strconv.ParseUint(fields[2], 10, 64)

		dc.data.stats[iocoreName] = data
	} // end of scanner.Scan()

	return nil
}

func (dc *IOCoreCollector) getNumOfIOCores(vhostPath string) (int, error) {
	content, err := ioutil.ReadFile(path.Join(vhostPath, "nr_iocores"))
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(content))

	nr_iocores, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, err
	}

	return nr_iocores, nil
}


// calcDerivatives calculates derivatives of metrics values and store them in IOCoreCollector structure as a 'output'
func (dc *IOCoreCollector) calcDerivatives() error {
	var diffVal uint64
	data := make(map[string]uint64)

	for iocore, values := range dc.data.stats {
		if _, present := dc.dataPrev.stats[iocore]; !present {
			continue
		}

		for key, val := range values {
			/** Calculate the change of the value in interval time **/
			valPrev := dc.dataPrev.stats[iocore][key]

			// if the counter wraps around
			if val < valPrev {
				diffVal = 1 + val + (uintMax - valPrev)
			} else {
				diffVal = val - valPrev
			}

			data[key] = diffVal
		}

		util := 100.0 * float64(data[nTotalWorkCycles]) / float64(data[nTotalCycles])
		util = math.Max(0, math.Min(100, util))
		dc.output[iocore+"/"+nCpuUtilization] = float64(int(util * 10)) / 10.0
	}

	var total_util float64 = 0
	for _, val := range dc.output {
		total_util += val
	}
	dc.output["cpu/"+nCpuUtilization] = total_util 

	return nil
}

// stashData copies iocoreStats struct variables items with their values from 'src' to 'dst'
func stashData(dst *iocoreStats, src *iocoreStats) {
	dst.timestamp = src.timestamp

	// copy map, deep copy is needed
	for key, value := range src.stats {
		dst.stats[key] = value
	}
}

// createNamespace returns namespace slice of strings composed from: vendor, class, type and components of metric name
func createNamespace(name string) []string {
	return append(prefix, strings.Split(name, "/")...)
}

// parseNamespace performs reverse operation to createNamespace, extracts metric key from namespace
func parseNamespace(ns []string) string {
	// skip prefix in namespace
	metric := ns[len(prefix):]
	return strings.Join(metric, "/")
}

func getVhostPath(cfg interface{}) (string, error) {
	path, _ := config.GetConfigItem(cfg, "vhost_path")
	
	return path.(string), nil
}
