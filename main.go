package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	_ "github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("expected 'create-user' or 'delete-user' subcommand")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create-user":
		createUserCmd := flag.NewFlagSet("create-user", flag.ExitOnError)
		newUserName := createUserCmd.String("username", "", "New user's username")
		newUserPassword := createUserCmd.String("password", "", "New user's password")
		permissionLevel := createUserCmd.String("permission", "Admin", "Permission level: Admin, Read, Write")
		roleArn := createUserCmd.String("role", "", "Role ARN to assume")
		resourceArn := createUserCmd.String("resource", "", "Resource ARN for the RDS cluster")
		secretArn := createUserCmd.String("secret", "", "Secret ARN to retrieve database credentials")
		databaseName := createUserCmd.String("database", "", "Database name")

		createUserCmd.Parse(os.Args[2:])

		if *newUserName == "" || *newUserPassword == "" || *roleArn == "" || *resourceArn == "" || *secretArn == "" || *databaseName == "" {
			log.Fatalf("username, password, role, resource, secret ARNs, and database name must be provided")
		}

		err := executeSQLStatements(*roleArn, *resourceArn, *secretArn, *databaseName, createUserSQLStatements(*newUserName, *newUserPassword, *databaseName, *permissionLevel))
		if err != nil {
			log.Fatalf("Error executing create user statements: %v", err)
		}

	case "delete-user":
		deleteUserCmd := flag.NewFlagSet("delete-user", flag.ExitOnError)
		userName := deleteUserCmd.String("username", "", "Username to delete")
		roleArn := deleteUserCmd.String("role", "", "Role ARN to assume")
		resourceArn := deleteUserCmd.String("resource", "", "Resource ARN for the RDS cluster")
		secretArn := deleteUserCmd.String("secret", "", "Secret ARN to retrieve database credentials")
		databaseName := deleteUserCmd.String("database", "", "Database name")

		deleteUserCmd.Parse(os.Args[2:])

		if *userName == "" || *roleArn == "" || *resourceArn == "" || *secretArn == "" || *databaseName == "" {
			log.Fatalf("username, role, resource, secret ARNs, and database name must be provided")
		}

		err := executeSQLStatements(*roleArn, *resourceArn, *secretArn, *databaseName, deleteUserSQLStatements(*userName, *databaseName))
		if err != nil {
			log.Fatalf("Error executing delete user statements: %v", err)
		}

	default:
		fmt.Println("expected 'create-user' or 'delete-user' subcommand")
		os.Exit(1)
	}
}

func executeSQLStatements(roleArn, resourceArn, secretArn, databaseName string, sqlStatements []string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	assumedRoleCredentials := stscreds.NewAssumeRoleProvider(stsClient, roleArn)

	customCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(aws.NewCredentialsCache(assumedRoleCredentials)))
	if err != nil {
		return fmt.Errorf("unable to load custom SDK config: %w", err)
	}

	svc := rdsdata.NewFromConfig(customCfg)

	for _, sqlStatement := range sqlStatements {
		input := &rdsdata.ExecuteStatementInput{
			SecretArn:   aws.String(secretArn),
			ResourceArn: aws.String(resourceArn),
			Sql:         aws.String(sqlStatement),
			Database:    aws.String(databaseName),
		}

		resp, err := svc.ExecuteStatement(context.TODO(), input)
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				log.Printf("Error code: %s, Message: %s, Fault: %s", apiErr.ErrorCode(), apiErr.ErrorMessage(), apiErr.ErrorFault().String())
			} else {
				log.Printf("Unexpected error: %v", err)
			}
			return fmt.Errorf("failed to execute statement: %w", err)
		}

		fmt.Printf("Statement executed successfully: %s\n", sqlStatement)
		if len(resp.Records) > 0 {
			fmt.Println("Response Records:")
			for _, record := range resp.Records {
				fmt.Println(record)
			}
		} else {
			fmt.Println("No response records.")
		}
	}
	return nil
}

func createUserSQLStatements(newUserName, newUserPassword, database, permissionLevel string) []string {
	var sqlStatements []string
	switch permissionLevel {
	case "Admin":
		sqlStatements = []string{
			fmt.Sprintf("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN CREATE USER %s WITH PASSWORD '%s'; END IF; END $$;", newUserName, newUserName, newUserPassword),
			fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s;", database, newUserName),
		}
	case "Read":
		sqlStatements = []string{
			fmt.Sprintf("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN CREATE USER %s WITH PASSWORD '%s'; END IF; END $$;", newUserName, newUserName, newUserPassword),
			fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s;", database, newUserName),
			fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s;", newUserName),
			fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s;", newUserName),
			fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO %s;", newUserName),
		}
	case "Write":
		sqlStatements = []string{
			fmt.Sprintf("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN CREATE USER %s WITH PASSWORD '%s'; END IF; END $$;", newUserName, newUserName, newUserPassword),
			fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s;", database, newUserName),
			fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s;", newUserName),
			fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s;", newUserName),
			fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s;", newUserName),
		}
	default:
		log.Fatalf("invalid permission level: %v", permissionLevel)
	}
	return sqlStatements
}

func deleteUserSQLStatements(userName string, database string) []string {
	return []string{
		fmt.Sprintf("REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s;", database, userName),
		fmt.Sprintf("REVOKE ALL PRIVILEGES ON SCHEMA public FROM %s;", userName),
		fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM %s;", userName),
		fmt.Sprintf("DROP USER IF EXISTS %s;", userName),
	}
}
