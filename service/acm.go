package service

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/acm"
    acmTypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
    "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
    "github.com/devhsoj/awsum/internal/memory"
)

type ACM struct {
    client *acm.Client
}

func (svc *ACM) Client() *acm.Client {
    if svc == nil || svc.client == nil {
        fmt.Printf("acm service not initialized!")
        os.Exit(1)
    }

    return svc.client
}

func (svc *ACM) getAllCertificateSummaries(ctx context.Context) ([]acmTypes.CertificateSummary, error) {
    var (
        output    *acm.ListCertificatesOutput
        certs     []acmTypes.CertificateSummary
        nextToken *string
        err       error
    )

    for {
        output, err = svc.Client().ListCertificates(ctx, &acm.ListCertificatesInput{
            NextToken: nextToken,
        })

        if err != nil {
            return nil, err
        }

        certs = append(certs, output.CertificateSummaryList...)
        nextToken = output.NextToken

        if nextToken == nil {
            break
        }
    }

    return certs, err
}

func (svc *ACM) GenerateLoadBalanceCertificateListFromCertificateNames(
    ctx context.Context,
    certificateNames []string,
) ([]types.Certificate, error) {
    if len(certificateNames) == 0 {
        return nil, nil
    }

    var certs []types.Certificate

    summaries, err := svc.getAllCertificateSummaries(ctx)

    if err != nil {
        return nil, err
    }

    for _, summary := range summaries {
        for _, certName := range certificateNames {
            // if cert name matches cert domain name
            if strings.Contains(strings.ToLower(memory.Unwrap(summary.DomainName)), strings.ToLower(certName)) {
                certs = append(certs, types.Certificate{
                    CertificateArn: summary.CertificateArn,
                })

                break
            }
        }
    }

    return certs, nil
}

func NewACM(awsConfig aws.Config) *ACM {
    return &ACM{
        client: acm.NewFromConfig(awsConfig),
    }
}
