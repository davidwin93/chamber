curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"name":"nginx","image":"nginx","dstPort":"80","protocol":"tcp"}' \
  http://localhost:8070/create
