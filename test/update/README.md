# Update Test

Validates that fields that were previously set by the user (e.g. target, auth), remain intact after an update.

## Expected

After running the `UPDATE` command, the `expected.yaml` should remain unchanged.
The `tag` field for `some/image` should be updated from `v1.0.0` to `v2.0.0`

See `acceptance.bats` for the test execution
