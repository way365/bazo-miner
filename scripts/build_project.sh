#!/bin/bash

# This script builds the bazo-miner project.
echo Downloading dependencies...
go get
echo Done.

# We need to fix all the imports if we forked the project.
./scripts/fix_imports.sh

echo Building the project...
go build
echo Done.

echo You can now start the miner. To start it type \"bazo-miner start\" or follow the instructions in the README file.
echo Enjoy!