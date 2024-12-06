build:
	go build ../

test_cli:
	go build ../
	../scheduler0 start

circle_ci_yaml_validation:
	brew upgrade circleci

execute_circle_ci_job:
	circleci local execute --job build


clean_test_cache:
	go clean -testcache

copy_build_into_e2e:
	go build -o ../ scheduler0
	cp ../scheduler0 ../e2e/node1/
	cp ../scheduler0 ../e2e/node2/
	cp ../scheduler0 ../e2e/node3/
	cp ../scheduler0 ../e2e/node4/
	cp ../scheduler0 ../e2e/node5/


start_e2e_server1:
	../e2e/node1/scheduler0 config init
	rm -rfd ../e2e/node1/raft_data
	rm -rfd ../raft_data/1
	../e2e/node1/scheduler0 start

start_e2e_server2:
	../e2e/node2/scheduler0 config init
	rm -rfd ../e2e/node2/raft_data
	rm -rfd ../raft_data/2
	../e2e/node2/scheduler0 start

start_e2e_server3:
	../e2e/node3/scheduler0 config init
	rm -rfd ../e2e/node3/raft_data
	rm -rfd ../raft_data/3
	../e2e/node3/scheduler0 start

start_e2e_server4:
	../e2e/node4/scheduler0 config init
	rm -rfd ../e2e/node4/raft_data
	rm -rfd ../raft_data/4
	../e2e/node4/scheduler0 start

start_e2e_server5:
	rm -rfd ../e2e/node5/raft_data
	rm -rfd ../raft_data/5
	../e2e/node5/scheduler0 start
