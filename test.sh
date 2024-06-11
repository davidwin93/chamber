curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"name":"nginx","image":"nginx","dstPort":"80","protocol":"tcp"}' \
  http://localhost:8070/vm

curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"name":"go-httpbin","image":"mccutchen/go-httpbin","dstPort":"8080","protocol":"tcp"}' \
  http://localhost:8070/vm

curl --header "Content-Type: application/json" \
  --request POST \
  --data '{"name":"httpbin","image":"kennethreitz/httpbin","dstPort":"80","protocol":"tcp"}' \
  http://localhost:8070/vm
