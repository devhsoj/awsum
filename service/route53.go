package service

import (
    "context"
    "errors"
    "fmt"
    "os"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/route53/types"
    "github.com/aws/aws-sdk-go-v2/service/route53"
    "github.com/levelshatter/awsum/internal/memory"
)

type Route53 struct {
    client *route53.Client
}

func (svc *Route53) GetAllHostedZones(ctx context.Context) ([]types.HostedZone, error) {
    var (
        output      *route53.ListHostedZonesOutput
        hostedZones []types.HostedZone
        marker      *string
        err         error
    )

    for {
        output, err = svc.Client().ListHostedZones(ctx, &route53.ListHostedZonesInput{
            Marker: marker,
        })

        if err != nil {
            return nil, err
        }

        hostedZones = append(hostedZones, output.HostedZones...)
        marker = output.Marker

        if marker == nil {
            break
        }
    }

    return hostedZones, err
}

func (svc *Route53) GetAllHostedZoneARecords(ctx context.Context, hostedZoneId string) ([]types.ResourceRecordSet, error) {
    var (
        output     *route53.ListResourceRecordSetsOutput
        records    []types.ResourceRecordSet
        identifier *string
        err        error
    )

    for {
        output, err = svc.Client().ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
            HostedZoneId:          memory.Pointer(hostedZoneId),
            StartRecordIdentifier: identifier,
            StartRecordType:       "A",
        })

        if err != nil {
            return nil, err
        }

        records = append(records, output.ResourceRecordSets...)
        identifier = output.NextRecordIdentifier

        if identifier == nil {
            break
        }
    }

    return records, err
}

func (svc *Route53) GetAssumedHostedZoneByDomainName(ctx context.Context, domainName string, private bool) (*types.HostedZone, error) {
    zones, err := svc.GetAllHostedZones(ctx)

    if err != nil {
        return nil, err
    }

    for _, zone := range zones {
        domainParts := strings.Split(domainName, ".")

        if len(domainParts) < 2 {
            return nil, errors.New("bad domain name")
        }

        assumedHostedZoneName := strings.Join([]string{
            domainParts[len(domainParts)-2],
            domainParts[len(domainParts)-1],
            "",
        }, ".")

        fmt.Println(memory.Unwrap(zone.Name), assumedHostedZoneName)

        if memory.Unwrap(zone.Name) == assumedHostedZoneName && memory.Unwrap(zone.Config).PrivateZone == private {
            return &zone, nil
        }
    }

    return nil, nil
}

type AttachDomainsToLoadBalancerOptions struct {
    Ctx              context.Context
    LoadBalancerName string
    ELBService       *ELBv2
    EC2Service       *EC2
    Private          bool
    DomainNames      []string
}

func (svc *Route53) AttachDomainsToLoadBalancer(opts AttachDomainsToLoadBalancerOptions) error {
    for _, domainName := range opts.DomainNames {
        hostedZone, err := svc.GetAssumedHostedZoneByDomainName(opts.Ctx, domainName, opts.Private)

        if err != nil {
            return err
        }

        if hostedZone == nil {
            return errors.New("hosted zone not found")
        }

        loadBalancer, err := opts.ELBService.SearchForLoadBalancerByName(opts.Ctx, opts.LoadBalancerName)

        if err != nil {
            return err
        }

        if loadBalancer == nil {
            return errors.New("load balancer not found")
        }

        _, err = svc.Client().ChangeResourceRecordSets(opts.Ctx, &route53.ChangeResourceRecordSetsInput{
            ChangeBatch: &types.ChangeBatch{
                Changes: []types.Change{
                    {
                        Action: "UPSERT",
                        ResourceRecordSet: &types.ResourceRecordSet{
                            Name: memory.Pointer(domainName),
                            Type: "A",
                            AliasTarget: &types.AliasTarget{
                                DNSName:              loadBalancer.DNSName,
                                HostedZoneId:         loadBalancer.CanonicalHostedZoneId,
                                EvaluateTargetHealth: false,
                            },
                        },
                    },
                },
                Comment: memory.Pointer("managed by awsum"),
            },
            HostedZoneId: hostedZone.Id,
        })

        if err != nil {
            return err
        }
    }

    return nil
}

func NewRoute53(awsConfig aws.Config) *Route53 {
    return &Route53{
        client: route53.NewFromConfig(awsConfig),
    }
}

func (svc *Route53) Client() *route53.Client {
    if svc == nil || svc.client == nil {
        fmt.Printf("route53 service not initialized!")
        os.Exit(1)
    }

    return svc.client
}
