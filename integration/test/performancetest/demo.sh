for commitCount in {0..12}
do
    echo "Demo-commit-#-$commitCount"
    sed -i -e "s/\(commitCount=\).*/\1$commitCount/" demo.txt
    git add demo.txt
    git commit -m"demo-$commitCount" 
    git push
    for i in {1..40}
    do
        echo "$i"
        sleep 1m
    done
    
done