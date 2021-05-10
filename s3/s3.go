package s3

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func NewS3Client() Client {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-central-1")},
	)
	if err != nil {
		panic(err)
	}

	return s3Client{
		sess: s3.New(sess),
	}
}

type s3Client struct {
	sess *s3.S3
}

func (a s3Client) CreateBucket(bucketName string) (*s3.CreateBucketOutput, error) {
	resp, err := a.sess.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		return nil, err
	}

	// Wait until bucket is created before finishing
	fmt.Printf("Waiting for bucket %q to be created...\n", bucketName)

	err = a.sess.WaitUntilBucketExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		return nil, err
	}
	fmt.Printf("Bucket %q successfully created\n", bucketName)
	return resp, nil
}

//func (a s3Client) Delete() error {}

type Client interface {
	CreateBucket(string) (*s3.CreateBucketOutput, error)

	//Delete() error
}
