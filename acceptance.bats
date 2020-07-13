#!/usr/bin/env bats

setup() {
  docker rmi quay.io/coreos/prometheus-operator:v0.40.0 -f
  docker rmi jimmidyson/configmap-reload:v0.3.0 -f
  docker rmi quay.io/coreos/prometheus-config-reloader:v0.40.0 -f
}

#@test "Pull command pulls all example images" {
#  run ./sinker pull source --manifest example
#  [[ "$output" =~ "All images have been pulled!" ]]
#}

@test "Push" {
  run ./sinker push --manifest test/e2e
  [[ "$output" =~ "All images have been pushed!" ]]
}

@test "List source matches example source list" {
  run ./sinker list source --manifest example --output example/source.txt
}

@test "List target matches example target list" {
  run ./sinker list target --manifest example --output example/target.txt
}
