#!/bin/sh
# Check: echo-value
# Returns the value of CHECK_PARAM_VALUE as actual output.
# Usage: `check:echo-value(value=hello)`

echo "${CHECK_PARAM_VALUE}"
