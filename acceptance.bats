#!/usr/bin/env bats

@test "[CHECK] Outdated image returns new image" {
  run ./sinker check --manifest example
  [[ "$output" =~ "New versions for" ]]
}

@test "[CHECK] Using --images flag returns newer versions" {
  run ./sinker check --images plexsystems/busybox:1.30.0 
  [[ "$output" =~ "New versions for" ]]
}

@test "[CREATE] Output matches example" {
  run ./sinker create example/bundle.yaml --target mycompany.com/myrepo --manifest example
  git diff --quiet -- example/.images.yaml
}

@test "[LIST] Source matches example source list" {
  run ./sinker list source --manifest example --output example/source.txt
  git diff --quiet -- example/source.txt
}

@test "[LIST] Target matches example target list" {
  run ./sinker list target --manifest example --output example/target.txt
  git diff --quiet -- example/target.txt
}

@test "[PUSH] --dryrun flag lists missing images" {
  run ./sinker push --dryrun --manifest test/manifests/dryrun-images.yaml
  [[ "$output" =~ "Image busybox:1.32.0 would be pushed as plexsystems/busybox:1.32.0" ]]
}

@test "[PUSH] All images are pushed" {
  run ./sinker push --manifest test/manifests/latest-images.yaml
  [[ "$output" =~ "All images have been pushed!" ]]
}

@test "[PUSH] All images with digests are pushed" {
  run ./sinker push --manifest test/manifests/digest-images.yaml
  [[ "$output" =~ "All images are up to date!" ]]
}

@test "[PULL] Source pulls all example images" {
  docker rmi jimmidyson/configmap-reload:v0.3.0 -f
  docker rmi quay.io/coreos/prometheus-config-reloader:v0.39.0 -f
  docker rmi quay.io/coreos/coreos/prometheus-operator:v0.40.0 -f
  run ./sinker pull source --manifest example
  [[ "$output" =~ "All images have been pulled!" ]]
  docker inspect jimmidyson/configmap-reload:v0.3.0
  docker inspect quay.io/coreos/prometheus-config-reloader:v0.39.0
  docker inspect quay.io/coreos/prometheus-operator:v0.40.0
}
