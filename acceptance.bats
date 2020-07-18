#!/usr/bin/env bats

@test "[CHECK] Using manifest returns newer image" {
  run ./sinker check --manifest example
  [[ "$output" =~ "New versions for" ]]
}

@test "[CHECK] Using --images flag returns newer versions" {
  run ./sinker check --images plexsystems/busybox:1.30.0 
  [[ "$output" =~ "New versions for" ]]
}

@test "[CREATE] Autodetection creates manifest that matches the example" {
  run ./sinker create example/bundle.yaml --target mycompany.com/myrepo --manifest example
  git diff --quiet -- example/.images.yaml
}

@test "[UPDATE] Updating manifest matches expected manifest" {
  run ./sinker update test/update/bundle.yaml --manifest test/update/original.yaml --output test/update/expected.yaml
  git diff --quiet -- test/update/updated-manifest.yaml
}

@test "[LIST] List of source images matches example source list" {
  run ./sinker list source --manifest example --output example/source.txt
  git diff --quiet -- example/source.txt
}

@test "[LIST] List of target images matches example target list" {
  run ./sinker list target --manifest example --output example/target.txt
  git diff --quiet -- example/target.txt
}

@test "[PUSH] Using --dryrun flag lists missing images" {
  run ./sinker push --dryrun --manifest test/push/dryrun-images.yaml
  [[ "$output" =~ "Image busybox:1.32.0 would be pushed as plexsystems/busybox:1.32.0" ]]
}

@test "[PUSH] Using manifest all latest images successfully pushed" {
  run ./sinker push --manifest test/push/latest-images.yaml
  [[ "$output" =~ "All images have been pushed!" ]]
}

@test "[PUSH] Using --images flag all latest images successfully pushed" {
  run ./sinker push --images busybox:latest --target plexsystems
  [[ "$output" =~ "All images have been pushed!" ]]
}

@test "[PUSH] All images with digests successfully pushed" {
  run ./sinker push --manifest test/push/digest-images.yaml
  [[ "$output" =~ "All images are up to date!" ]]
}

@test "[PULL] Using manifest pulls all example images" {
  docker rmi jimmidyson/configmap-reload:v0.3.0 -f
  docker rmi quay.io/coreos/prometheus-config-reloader:v0.39.0 -f
  docker rmi quay.io/coreos/coreos/prometheus-operator:v0.40.0 -f

  run ./sinker pull source --manifest example
  [[ "$output" =~ "All images have been pulled!" ]]

  docker inspect jimmidyson/configmap-reload:v0.3.0
  docker inspect quay.io/coreos/prometheus-config-reloader:v0.39.0
  docker inspect quay.io/coreos/prometheus-operator:v0.40.0
}

@test "[PULL] Using --images flag pulls all example images" {
  docker rmi jimmidyson/configmap-reload:v0.3.0 -f
  docker rmi quay.io/coreos/prometheus-config-reloader:v0.39.0

  run ./sinker pull --images jimmidyson/configmap-reload:v0.3.0,quay.io/coreos/prometheus-config-reloader:v0.39.0
  [[ "$output" =~ "All images have been pulled!" ]]

  docker inspect jimmidyson/configmap-reload:v0.3.0
  docker inspect quay.io/coreos/prometheus-config-reloader:v0.39.0
}
