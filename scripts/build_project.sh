#!/bin/bash

# This script builds the bazo-miner project.

# First, we need to fix all the imports if we forked the project.
./fix_imports.sh

# Then we build the project
echo Building the project...

cd ..
go get && go build

echo Done.
echo You can now start the miner. To start it type \"bazo-miner start\" or follow the instructions in the README file.
echo Enjoy...