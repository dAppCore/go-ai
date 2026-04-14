package main

import (
	"reflect"
	"testing"
)

func TestBuildBenchmarkModelNames_Good_DefaultsPlusInstalledExtras(t *testing.T) {
	installedModelNames := []string{
		"embeddinggemma:latest",
		"nomic-embed-text:latest",
		"mxbai-embed-large:latest",
		"snowflake-arctic-embed2:335m",
		"mxbai-embed-large:latest",
	}

	got := buildBenchmarkModelNames(installedModelNames)
	want := []string{
		"nomic-embed-text",
		"embeddinggemma",
		"mxbai-embed-large:latest",
		"snowflake-arctic-embed2:335m",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildBenchmarkModelNames() = %v, want %v", got, want)
	}
}

func TestHasInstalledModel_Good_DefaultMatchesTaggedInstall(t *testing.T) {
	installedModelNames := []string{
		"nomic-embed-text:latest",
		"mxbai-embed-large:latest",
	}

	if !hasInstalledModel(installedModelNames, "nomic-embed-text") {
		t.Fatal("expected default model to match installed :latest tag")
	}
	if hasInstalledModel(installedModelNames, "embeddinggemma") {
		t.Fatal("expected missing model to return false")
	}
}

func TestDecodeInstalledModelNames_Good(t *testing.T) {
	got, err := decodeInstalledModelNames([]byte(`{
		"models": [
			{"name": "nomic-embed-text:latest"},
			{"name": "snowflake-arctic-embed2:335m"},
			{"name": ""}
		]
	}`))
	if err != nil {
		t.Fatalf("decodeInstalledModelNames(): %v", err)
	}

	want := []string{
		"nomic-embed-text:latest",
		"snowflake-arctic-embed2:335m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeInstalledModelNames() = %v, want %v", got, want)
	}
}
