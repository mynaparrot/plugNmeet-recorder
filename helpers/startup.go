package helpers

import (
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/factory"
	"gopkg.in/yaml.v3"
	"os"
)

func ReadYamlConfigFile(file string) (*config.AppConfig, error) {
	yamlFile, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	appCnf := new(config.AppConfig)
	err = yaml.Unmarshal(yamlFile, &appCnf)
	if err != nil {
		return nil, err
	}

	// get current working dir
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// set the root path
	appCnf.RootWorkingDir = wd

	return appCnf, err
}

func PrepareServer(appCnf *config.AppConfig) error {
	// nats
	err := factory.NewNatsConnection(appCnf)
	if err != nil {
		return err
	}

	return nil
}
