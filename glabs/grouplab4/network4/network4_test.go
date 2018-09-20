package network4

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/uis-dat520-s18/glabs/grouplab4/logger4"
)

func StartNetworkAddsConnectionsToConnectionsMap(t *testing.T) {

	logger := &logger4.Logger{}

	config, err := loadconfig("development")

	net1, err := CreateNetwork(config, 1, logger)
	if err != nil {
		logger.Log("system", "crit", "#Application: Failed to create network with error: "+err.Error())
		os.Exit(0)
	}

	net2, err := CreateNetwork(config, 2, logger)
	if err != nil {
		logger.Log("system", "crit", "#Application: Failed to create network with error: "+err.Error())
		os.Exit(0)
	}

	net1.StartNetwork()

	net2.StartNetwork()

	fmt.Printf("%v", net1.Connections)

	if net1.Connections[2] == nil {
		t.Errorf("Net2 id not added to net1")
	}

	net1.CleanupNetwork()
	net2.CleanupNetwork()

}

func loadconfig(mode string) (ConfigInstance, error) {
	file, err := os.Open("../config.json")
	defer file.Close()
	if err != nil {
		return ConfigInstance{}, err
	}
	decoder := json.NewDecoder(file)
	config := Config{}
	err = decoder.Decode(&config) //invalid argument
	if err != nil {
		return ConfigInstance{}, err
	}
	var conf ConfigInstance
	if mode == "production" {
		conf = config.Production
	} else {
		conf = config.Development
	}
	return conf, nil
}
