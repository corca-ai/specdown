#!/bin/sh
# Fixture: echo-value
# Returns the value of FIXTURE_PARAM_VALUE as actual output.
# Usage: `fixture:echo-value(value=hello)`

echo "${FIXTURE_PARAM_VALUE}"
