export OLD="true"
startDate=${1:-1627262978}
endDate=${2:-1658798978}
echo $startDate $endDate
go run  --tags=generator integration/generator/test_case_generator.go  $startDate $endDate


git add integration/generator/resources/ec2_performance_old_test_matrix.json
git commit -m "Testing releases from $startDate to $endDate"
git push