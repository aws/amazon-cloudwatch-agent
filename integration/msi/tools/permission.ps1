$wrongsddl="D:AI(A;OICIID;FA;;;SY)(A;OICIID;FA;;;BA)(A;OICIIOID;GA;;;CO)(A;OICIID;0x1200a9;;;BU)(A;CIID;DCLCRPCR;;;BU)"
$output=& cacls.exe "${env:ProgramData}\Amazon\AmazonCloudWatchAgent" /S
$currsddl=$output.Split('"')[1]

If ($currsddl -eq $wrongsddl) {
    & echo Y| cacls "${env:ProgramData}\Amazon\AmazonCloudWatchAgent" /S:"D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICIIO;FA;;;CO)(A;OICI;GR;;;BU)"
}