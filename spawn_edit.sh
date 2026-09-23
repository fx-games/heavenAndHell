#!/bin/sh

cd tools/spawnedit && go run . -mod "${1:-heavenandhell}"
