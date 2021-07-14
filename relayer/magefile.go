//+build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func Build() {
	mg.Deps(BuildMain)
}

func BuildMain() error {
	return sh.Run("go", "build", "-o", "build/artemis-relay", "main.go")
}

func Test() error {
	return sh.RunV("go", "test", "./...")
}

func Lint() error {
	return sh.Run("revive", "-config", "revive.toml", "./...")
}

func Install() error {
	return sh.Run("go", "build", "-o", "$GOPATH/bin/artemis-relay", "main.go")
}

func SubBeef() error {
	cmd := "go"
	env := map[string]string{
		"SNOWBRIDGE_BEEFY_KEY":      "0x935b65c833ced92c43ef9de6bff30703d941bd92a2637cb00cfad389f5862109",
		"SNOWBRIDGE_MESSAGE_KEY":    "0x8013383de6e5a891e7754ae1ef5a21e7661f1fe67cd47ca8ebf4acd6de66879a",
		"SNOWBRIDGE_PARACHAIN_KEY":  "//Relay",
		"SNOWBRIDGE_RELAYCHAIN_KEY": "//Alice",
	}
	return sh.RunWithV(env, cmd, "run", "./main.go", "sub-beefy", "--config", "/tmp/snowbridge-e2e-config/config.toml")
}

func Dev() error {
	cmd := "go"
	env := map[string]string{
		"SNOWBRIDGE_BEEFY_KEY":      "0x935b65c833ced92c43ef9de6bff30703d941bd92a2637cb00cfad389f5862109",
		"SNOWBRIDGE_MESSAGE_KEY":    "0x8013383de6e5a891e7754ae1ef5a21e7661f1fe67cd47ca8ebf4acd6de66879a",
		"SNOWBRIDGE_PARACHAIN_KEY":  "//Relay",
		"SNOWBRIDGE_RELAYCHAIN_KEY": "//Alice",
	}
	return sh.RunWithV(env, cmd, "run", "./main.go", "run", "--config", "/tmp/snowbridge-e2e-config/config.toml")
}
