package main

type Settings struct {
	Environment   string `yaml:"ENVIRONMENT"`
	CompassAPIKey string `yaml:"COMPASS_API_KEY"`
	ConsentEmail  string `yaml:"CONSENT_EMAIL"`
}
