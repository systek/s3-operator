package s3

func NewS3Client(accessKey string) Client {
	return s3Client{}

}

type s3Client struct {
}

func (a s3Client) Create() {

}

func (a s3Client) Delete() {

}

type Client interface {
	Create()

	Delete()
}
