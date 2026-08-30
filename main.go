package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/pelletier/go-toml/v2"
)

type configFile struct {
	SQL  sqlConfig `toml:"sql"`
	Key  keyConfig `toml:"key"`
	Keys keyConfig `toml:"keys"`
}

type sqlConfig struct {
	User                  string `toml:"user"`
	Password              string `toml:"password"`
	Address               string `toml:"address"`
	Name                  string `toml:"name"`
	BBoltDatabaseFileName string `toml:"bbolt_database_file_name"`
}

type keyConfig struct {
	MapServiceAPIKey       string `toml:"map_service_api_key"`
	AmapServiceAccessToken string `toml:"amap_service_access_token"`
	SmsAccessToken         string `toml:"sms_access_token"`
	SmsTemplateCode        string `toml:"sms_template_code"`
}

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("config file %q not found; using built-in defaults", path)
			return nil
		}
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	var config configFile
	if err := toml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}
	applySQLConfig(config.SQL)
	applyKeyConfig(config.Key)
	applyKeyConfig(config.Keys)
	return nil
}

func applySQLConfig(config sqlConfig) {
	if value := strings.TrimSpace(config.User); value != "" {
		environment.MySqlDatabaseUser = value
	}
	if config.Password != "" {
		environment.MySqlDatabasePassword = config.Password
	}
	if value := strings.TrimSpace(config.Address); value != "" {
		environment.MySqlDatabaseAddress = value
	}
	if value := strings.TrimSpace(config.Name); value != "" {
		environment.MySqlDatabaseName = value
	}
	if value := strings.TrimSpace(config.BBoltDatabaseFileName); value != "" {
		environment.BBoltDatabaseFileName = value
	}
}

func applyKeyConfig(config keyConfig) {
	if value := strings.TrimSpace(config.MapServiceAPIKey); value != "" {
		utils.MapServiceAPIKey = value
	}
	if value := strings.TrimSpace(config.AmapServiceAccessToken); value != "" {
		utils.AmapServiceAccessToken = value
	}
	if value := strings.TrimSpace(config.SmsAccessToken); value != "" {
		utils.SmsAccessToken = value
	}
	if value := strings.TrimSpace(config.SmsTemplateCode); value != "" {
		utils.SmsTemplateCode = value
	}
}

func main() {
	if err := loadConfig("config.toml"); err != nil {
		log.Fatal(err)
	}
	environment.Initialize()
	router := service.InitAndMakeRouter()
	router.Run(fmt.Sprintf(":%d", 80))
}
