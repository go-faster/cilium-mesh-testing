#!/bin/bash

until $(curl --output /dev/null --silent --fail ${1}); do
    sleep 5
done