#!/bin/bash

docker-compose run --rm client
exit_code=$?
docker-compose down
exit $exit_code
