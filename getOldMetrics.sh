export OLD="true"
startDate=${1:-1627262978}
endDate=${2:-1658798978}
echo $startDate $endDate
go run  --tags=generator integration/generator/test_case_generator.go  $startDate $endDate

