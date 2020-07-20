#!/usr/bin/env bats

@test "[CHECK] Using manifest returns newer image" {
  run ./sinker check --manifest example
  [[ "$output" =~ "New versions for" ]]
}

@test "[CHECK] Using --images flag returns newer versions" {
  run ./sinker check --images plexsystems/sinker-test:0.0.1
  [[ "$output" =~ "New versions for" ]]
}

@test "[CREATE] New manifest with autodetection creates example manifest" {
  run ./sinker create example/bundle.yaml --target mycompany.com/myrepo --manifest example/output.yaml

  if !(cmp -s "example/.images.yaml" "example/output.yaml"); then
    rm example/output.yaml
    return 1
  fi

  rm example/output.yaml
}

@test "[UPDATE] Updating manifest matches expected manifest" {
  run ./sinker update test/update/bundle.yaml --manifest test/update/original.yaml --output test/update/expected.yaml
  git diff --quiet -- test/update/expected.yaml
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
  run ./sinker push --dryrun --manifest test/push
  [[ "$output" =~ "Image busybox:latest would be pushed as plexsystems/busybox:latest" ]]
}

@test "[PUSH] Using manifest all latest images successfully pushed" {
  run ./sinker push --manifest test/push
  [[ "$output" =~ "All images have been pushed!" ]]
}

@test "[PUSH] Using --images flag all latest images successfully pushed" {
  run ./sinker push --images busybox:latest --target plexsystems
  [[ "$output" =~ "All images have been pushed!" ]]
}

@test "[PULL] Using manifest pulls all images" {
  docker rmi plexsystems/sinker-test:latest -f
  docker rmi plexsystems/sinker-test:1.0.0 -f

  run ./sinker pull target --manifest test/pull
  [[ "$output" =~ "All images have been pulled!" ]]

  docker inspect plexsystems/sinker-test:latest
  docker inspect plexsystems/sinker-test:1.0.0
}

@test "[PULL] Using --images flag pulls all images" {
  docker rmi plexsystems/sinker-test:latest -f
  docker rmi plexsystems/sinker-test:1.0.0 -f

  run ./sinker pull --images plexsystems/sinker-test:latest,plexsystems/sinker-test:1.0.0
  [[ "$output" =~ "All images have been pulled!" ]]

  docker inspect plexsystems/sinker-test:latest
  docker inspect plexsystems/sinker-test:1.0.0
}
