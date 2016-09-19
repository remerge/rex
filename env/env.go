package env

import "os"

var Key = "REX_ENV"
var Env string

func init() {
	Set(os.Getenv(Key))
}

func Set(name string) {
	if name == "" {
		name = "development"
	}

	if err := os.Setenv(Key, name); err != nil {
		panic(err)
	} else {
		Env = name
	}
}

func IsProd() bool {
	return Env == "production" || Env == "prod"
}

func IsDevel() bool {
	return Env == "development" || Env == "devel" || Env == "dev"
}

func IsTesting() bool {
	return Env == "testing" || Env == "test"
}
