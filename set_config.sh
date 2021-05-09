#!/bin/sh
set -o errexit

kubectl set env deployment/s3-operator-controller-manager -n s3-operator-system AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}