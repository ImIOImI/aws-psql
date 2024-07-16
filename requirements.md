# AWS RDS User Management

This project provides CLI commands to manage PostgreSQL users in an AWS RDS instance using the AWS RDS Data API. 
The commands available are `create-user` and `delete-user`.

## Requirements

- Go 1.15 or later
- AWS CLI configured with SSO
- AWS IAM Role with necessary permissions
- AWS RDS Data API enabled for your RDS instance
- AWS Secrets Manager to store database credentials

## Setup

1. **Install Go**: Ensure Go is installed on your machine. You can download it from [golang.org](https://golang.org/dl/).

2. **Set Up AWS CLI**: Make sure the AWS CLI is configured and your SSO session is active.
   ```sh
   aws configure sso
   ```
3. Initialize Go Module: Initialize a new Go module in your project directory and download dependencies.
   ```shell
   go mod init aws-rds-user-management
   go get github.com/aws/aws-sdk-go-v2/config
   go get github.com/aws/aws-sdk-go-v2/service/rdsdata
   go get github.com/aws/aws-sdk-go-v2/service/sts
   go get github.com/aws/aws-sdk-go-v2/credentials/stscreds
   ```
## Commands
### Create User
Creates a new PostgreSQL user with specified permissions.

Usage
```shell
go run main.go create-user \
  -username=<username> \ 
  -password=<password> \
  -permission=<Admin|Read|Write> \
  -role=<role-arn> \
  -resource=<resource-arn> \
  -secret=<secret-arn> \
  -database=<database-name>
```

Example
```shell
go run main.go create-user \
  -username=new_user \
  -password=your_password \
  -permission=Admin \
  -role=arn:aws:iam::123456789012:role/YourRole \
  -resource=arn:aws:rds:us-east-1:123456789012:cluster:your-cluster-arn \
  -secret=arn:aws:secretsmanager:us-east-1:123456789012:secret:your-secret-arn \
  -database=your_database_name
```

### Delete User
Deletes a PostgreSQL user and revokes all permissions.
```shell
go run main.go delete-user \
  -username=<username> \
  -role=<role-arn> \
  -resource=<resource-arn> \
  -secret=<secret-arn> \
  -database=<database-name>
```

Example
```shell
go run main.go delete-user \
  -username=new_user \
  -role=arn:aws:iam::123456789012:role/YourRole \
  -resource=arn:aws:rds:us-east-1:123456789012:cluster:your-cluster-arn \
  -secret=arn:aws:secretsmanager:us-east-1:123456789012:secret:your-secret-arn \
  -database=your_database_name
```

## Permissions
Ensure the IAM role used has the necessary permissions to access the RDS Data API and AWS Secrets Manager. Example IAM 
Policy:

```shell
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "sts:AssumeRole"
            ],
            "Resource": "arn:aws:iam::123456789012:role/YourRole"
        },
        {
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetSecretValue",
                "rds-data:ExecuteStatement"
            ],
            "Resource": [
                "arn:aws:secretsmanager:us-east-1:123456789012:secret:your-secret-arn",
                "arn:aws:rds:us-east-1:123456789012:cluster:your-cluster-arn"
            ]
        }
    ]
}
```