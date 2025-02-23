#! /bin/sh
for CMD in `ls cmd`; do
  go build ./cmd/$CMD
done
