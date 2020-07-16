package commands

import (
	"reflect"
	"testing"

	"github.com/hashicorp/go-version"
)

func TestFilterTags(t *testing.T) {
	tags := []string{
		"noperiods",
		"contains-hypen",
		"1.0.0",
		"v1.0.0",
	}

	actual := filterTags(tags)
	expected := []string{"1.0.0", "v1.0.0"}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected filtering of tags. expected %v actual %v", expected, actual)
	}
}

func TestNewerVersions(t *testing.T) {
	currentTag, err := version.NewVersion("v0.1.0")
	if err != nil {
		t.Fatal("new version:", err)
	}

	foundTags := []string{"v1.0.0", "v2.0.0", "v3.0.0", "v4.0.0", "v5.0.0", "v6.0.0"}

	actual, err := getNewerVersions(currentTag, foundTags)
	if err != nil {
		t.Fatal("get newer versions:", err)
	}

	expected := []string{"...", "v2.0.0", "v3.0.0", "v4.0.0", "v5.0.0", "v6.0.0"}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected filtering of tags. expected %v actual %v", expected, actual)
	}
}
