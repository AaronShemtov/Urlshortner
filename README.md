# Urlshortner

aws lambda add-permission \
    --function-name Urlshortner \
    --statement-id apigateway-invoke \
    --action lambda:InvokeFunction \
    --principal apigateway.amazonaws.com \
    --source-arn "arn:aws:execute-api:eu-central-1:816069159445:k0meyaql4k/*"
