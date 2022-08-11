export OLD="true"
startDate=${1:-1622398650}
endDate=${2:-1653934651}
go run  --tags=generator integration/generator/test_case_generator.go  $startDate $endDate
cleanStart=$(date -d @$startDate +%Y/%m/%d)
cleanEnd=$(date -d @$endDate +%Y/%m/%d)
echo "$cleanStart to $cleanEnd"
git add integration/generator/resources/ec2_performance_old_test_matrix.json
git commit -m "Testing releases from $cleanStart to $cleanEnd"
git push