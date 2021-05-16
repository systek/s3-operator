package iam

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"html/template"
)


func NewIamClient() Client {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-central-1")},
	)
	if err != nil {
		panic(err)
	}

	return iamClient{
		sess: iam.New(sess),
	}
}

type iamClient struct {
	sess *iam.IAM
}

type Client interface {
	CreateAccessKey(string) (*iam.CreateAccessKeyOutput, error)
	CreateUser(string) (string, error)
	CreatePolicy(string, string, string) (*iam.CreatePolicyOutput, error)
	AttachPolicy(string, string) error
}


func (a iamClient) AttachPolicy(userName string, policyArn string) error {
	input := &iam.AttachUserPolicyInput{
		PolicyArn: aws.String(policyArn),
		UserName:  aws.String(userName),
	}

	result, err := a.sess.AttachUserPolicy(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeNoSuchEntityException:
				fmt.Println(iam.ErrCodeNoSuchEntityException, aerr.Error())
			case iam.ErrCodeLimitExceededException:
				fmt.Println(iam.ErrCodeLimitExceededException, aerr.Error())
			case iam.ErrCodeInvalidInputException:
				fmt.Println(iam.ErrCodeInvalidInputException, aerr.Error())
			case iam.ErrCodePolicyNotAttachableException:
				fmt.Println(iam.ErrCodePolicyNotAttachableException, aerr.Error())
			case iam.ErrCodeServiceFailureException:
				fmt.Println(iam.ErrCodeServiceFailureException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return nil
	}

	fmt.Println(result)
	return nil
}

func (a iamClient) CreatePolicy(policyName string, description string, bucketName string) (*iam.CreatePolicyOutput,error){
	anonBucketName := struct {
		BucketName string
	} {
		BucketName: bucketName,
	}
	policyDocument := `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "s3:*",
            "Resource": "arn:aws:s3:::{{.BucketName}}"
        }
    ]
}`
	t, err := template.New("PolicyDocument").Parse(policyDocument)
	if err != nil {
		fmt.Println("Could not parse template")
		return nil,err
	}
	var tpl bytes.Buffer
	err = t.Execute(&tpl, anonBucketName)
	input := &iam.CreatePolicyInput{
		Description:    aws.String(description),
		PolicyDocument: aws.String(tpl.String()),
		PolicyName:     aws.String(policyName),
	}
	result, err := a.sess.CreatePolicy(input)
	if err != nil {
		//TODO: handle error later
		return nil, err
	}
	fmt.Println("New Policy", result)
	return result, nil
}

func (a iamClient) CreateAccessKey(userName string) (*iam.CreateAccessKeyOutput,error) {
	input := &iam.CreateAccessKeyInput{
		UserName: aws.String(userName),
	}

	result, err := a.sess.CreateAccessKey(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeNoSuchEntityException:
				fmt.Println(iam.ErrCodeNoSuchEntityException, aerr.Error())
			case iam.ErrCodeLimitExceededException:
				fmt.Println(iam.ErrCodeLimitExceededException, aerr.Error())
			case iam.ErrCodeServiceFailureException:
				fmt.Println(iam.ErrCodeServiceFailureException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return nil, nil
	}

	fmt.Println(result)
	return result, nil
}

func (a iamClient) CreateUser(userName string) (string, error) {
	input := &iam.CreateUserInput{
		UserName: aws.String(userName),
	}

	result, err := a.sess.CreateUser(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeLimitExceededException:
				fmt.Println(iam.ErrCodeLimitExceededException, aerr.Error())
			case iam.ErrCodeEntityAlreadyExistsException:
				fmt.Println(iam.ErrCodeEntityAlreadyExistsException, aerr.Error())
			case iam.ErrCodeNoSuchEntityException:
				fmt.Println(iam.ErrCodeNoSuchEntityException, aerr.Error())
			case iam.ErrCodeInvalidInputException:
				fmt.Println(iam.ErrCodeInvalidInputException, aerr.Error())
			case iam.ErrCodeConcurrentModificationException:
				fmt.Println(iam.ErrCodeConcurrentModificationException, aerr.Error())
			case iam.ErrCodeServiceFailureException:
				fmt.Println(iam.ErrCodeServiceFailureException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return "", nil
	}

	return *result.User.UserName, nil
}