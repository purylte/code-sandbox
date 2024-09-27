air -c ./.air.toml & \
make watch-css & \
(cd builder && make) & \
(cd runner && make) 