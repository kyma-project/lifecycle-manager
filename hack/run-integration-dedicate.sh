#! /bin/bash

task(){
   sleep 0.5; KUBEBUILDER_ASSETS=$envtest go test  ./internal/controller/control-plane -ginkgo.v -ginkgo.focus-file $1
}

envtest=$1
for filename in ./internal/controller/control-plane/*; do
  task "$filename" &
done

