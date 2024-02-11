package cloudflaretunnel

import (
	"context"
	"fmt"
	"os"
	"strings"

	cloudflare "github.com/cloudflare/cloudflare-go"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
	"sigs.k8s.io/external-dns/provider/cloudflaretunnel/util"
)

type CloudFlareAPIClient interface {
	GetTunnelConfiguration(context.Context, *cloudflare.ResourceContainer, string) (cloudflare.TunnelConfigurationResult, error)
	UpdateTunnelConfiguration(context.Context, *cloudflare.ResourceContainer, cloudflare.TunnelConfigurationParams) (cloudflare.TunnelConfigurationResult, error)
	UserDetails(ctx context.Context) (cloudflare.User, error)
	ZoneIDByName(zoneName string) (string, error)
	ListZones(ctx context.Context, zoneID ...string) ([]cloudflare.Zone, error)
	ListZonesContext(ctx context.Context, opts ...cloudflare.ReqOption) (cloudflare.ZonesResponse, error)
	ZoneDetails(ctx context.Context, zoneID string) (cloudflare.Zone, error)
	ListDNSRecords(ctx context.Context, rc *cloudflare.ResourceContainer, rp cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error)
	CreateDNSRecord(ctx context.Context, rc *cloudflare.ResourceContainer, rp cloudflare.CreateDNSRecordParams) (cloudflare.DNSRecord, error)
	DeleteDNSRecord(ctx context.Context, rc *cloudflare.ResourceContainer, recordID string) error
	UpdateDNSRecord(ctx context.Context, rc *cloudflare.ResourceContainer, rp cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error)
}

type CloudFlareTunnelProvider struct {
	provider.BaseProvider
	Client            CloudFlareAPIClient
	domainFilter      endpoint.DomainFilter
	DryRun            bool
	accountId         string
	tunnelId          string
	zoneNameIDMapper  provider.ZoneIDName
	zoneIDFilter      provider.ZoneIDFilter
	DNSRecordsPerPage int
}

var defaultOriginRequestConfig = cloudflare.OriginRequestConfig{
	NoTLSVerify: boolPtr(true),
	Http2Origin: boolPtr(true),
}

func NewCloudFlareAPIClient() (CloudFlareAPIClient, error) {
	var (
		client *cloudflare.API
		err    error
	)

	token, ok := os.LookupEnv("CF_API_TOKEN")
	if ok {
		if strings.HasPrefix(token, "file:") {
			tokenBytes, err := os.ReadFile(strings.TrimPrefix(token, "file:"))
			if err != nil {
				return nil, fmt.Errorf("failed to read CF_API_TOKEN from file: %w", err)
			}
			token = string(tokenBytes)
		}
		client, err = cloudflare.NewWithAPIToken(token)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize cloudflare provider: %v", err)
		}
	} else {
		client, err = cloudflare.New(os.Getenv("CF_API_KEY"), os.Getenv("CF_API_EMAIL"))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize cloudflare provider: %v", err)
		}
	}
	return client, nil
}

func NewCloudFlareTunnelProvider(client CloudFlareAPIClient, domainFilter endpoint.DomainFilter, zoneIDFilter provider.ZoneIDFilter, dryRun bool, dnsRecordsPerPage int) (*CloudFlareTunnelProvider, error) {

	accountId, ok := os.LookupEnv("CF_ACCOUNT_ID")
	if !ok {
		return nil, fmt.Errorf("failed to get cloudflare account id: please set env, CF_ACCOUNT_ID")
	}

	tunnelId, ok := os.LookupEnv("CF_TUNNEL_ID")
	if !ok {
		return nil, fmt.Errorf("failed to get cloudflare tunnel id: please set env, CF_TUNNEL_ID")
	}

	provider := &CloudFlareTunnelProvider{
		Client:           client,
		accountId:        accountId,
		DryRun:           dryRun,
		tunnelId:         tunnelId,
		domainFilter:     domainFilter,
		zoneIDFilter:     zoneIDFilter,
		zoneNameIDMapper: provider.ZoneIDName{},
	}
	return provider, nil
}

func (p *CloudFlareTunnelProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	configResult, err := p.Client.GetTunnelConfiguration(ctx, cloudflare.AccountIdentifier(p.accountId), p.tunnelId)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnel configs: %v", err)
	}

	endpoints := []*endpoint.Endpoint{}
	for _, config := range configResult.Config.Ingress {
		endpoint := endpoint.NewEndpoint(config.Hostname, endpoint.RecordTypeA, config.Service)
		endpoints = append(endpoints, endpoint)
		log.Info(fmt.Sprintf("current endpoint: %v", endpoint))
	}
	return endpoints, nil
}

func (p *CloudFlareTunnelProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	err := p.updateZoneIdMapper(ctx)
	if err != nil {
		return fmt.Errorf("failed to update zoneidmapper: %v", err)
	}

	oldConfigResult, err := p.Client.GetTunnelConfiguration(ctx, cloudflare.AccountIdentifier(p.accountId), p.tunnelId)
	if err != nil {
		return fmt.Errorf("failed to get tunnel configs: %v", err)
	}

	ingressConfigs := util.NewOrderedMap(len(oldConfigResult.Config.Ingress))
	var catchAll cloudflare.UnvalidatedIngressRule

	for _, v := range oldConfigResult.Config.Ingress {
		if v.Hostname == "" {
			catchAll = v
			break
		}
		ingressConfigs.Add(v)
	}

	dnsCreateParams := make([]cloudflare.CreateDNSRecordParams, 0, len(changes.Create))
	for _, createEndpoint := range changes.Create {
		if createEndpoint.RecordType != endpoint.RecordTypeA {
			continue
		}
		ingressConfigs.Add(cloudflare.UnvalidatedIngressRule{
			Hostname:      createEndpoint.DNSName,
			Service:       convertHttps(createEndpoint.Targets[0]),
			OriginRequest: &defaultOriginRequestConfig,
		})

		zoneID, _ := p.zoneNameIDMapper.FindZone(createEndpoint.DNSName)
		if zoneID == "" {
			fmt.Println("zoneid is empty. skipping...")
			continue
		}

		dnsCreateParams = append(dnsCreateParams, cloudflare.CreateDNSRecordParams{
			Name:    createEndpoint.DNSName,
			TTL:     1, // auto
			Proxied: boolPtr(true),
			Type:    endpoint.RecordTypeCNAME,
			Content: fmt.Sprintf("%v.cfargotunnel.com", p.tunnelId),
			ZoneID:  zoneID,
		})
	}

	type dnsUpdateParam struct {
		ZoneID string
		cfup   cloudflare.UpdateDNSRecordParams
	}
	dnsUpdateParams := make([]dnsUpdateParam, 0, len(changes.UpdateNew))
	for _, desired := range changes.UpdateNew {
		if desired.RecordType != endpoint.RecordTypeA {
			continue
		}

		ingressConfigs.Update(cloudflare.UnvalidatedIngressRule{
			Hostname:      desired.DNSName,
			Service:       convertHttps(desired.Targets[0]),
			OriginRequest: &defaultOriginRequestConfig,
		})

		zoneID, _ := p.zoneNameIDMapper.FindZone(desired.DNSName)
		if zoneID == "" {
			fmt.Println("zoneid is empty. skipping...")
			continue
		}

		records, err := p.listDNSRecordsWithAutoPagination(ctx, zoneID)
		if err != nil {
			return err
		}
		recordID := p.getRecordID(records, cloudflare.DNSRecord{
			Name:    desired.DNSName,
			Type:    endpoint.RecordTypeCNAME,
			Content: desired.Targets[0],
			ZoneID:  zoneID,
		})

		dnsUpdateParams = append(dnsUpdateParams, dnsUpdateParam{
			ZoneID: zoneID,
			cfup: cloudflare.UpdateDNSRecordParams{
				ID:      recordID,
				Name:    desired.DNSName,
				TTL:     1, // auto
				Proxied: boolPtr(true),
				Type:    endpoint.RecordTypeCNAME,
				Content: fmt.Sprintf("%v.cfargotunnel.com", p.tunnelId),
			},
		})
	}

	type dnsDeleteParam struct {
		ZoneID   string
		recordID string
	}
	dnsDeleteParams := make([]dnsDeleteParam, 0, len(changes.Delete))
	for _, deleteEndpoint := range changes.Delete {
		if deleteEndpoint.RecordType != endpoint.RecordTypeA {
			continue
		}
		ingressConfigs.Remove(cloudflare.UnvalidatedIngressRule{
			Hostname: deleteEndpoint.DNSName,
		})

		zoneID, _ := p.zoneNameIDMapper.FindZone(deleteEndpoint.DNSName)
		if zoneID == "" {
			fmt.Println("zoneid is empty. skipping...")
			continue
		}

		records, err := p.listDNSRecordsWithAutoPagination(ctx, zoneID)
		if err != nil {
			return err
		}

		dnsDeleteParams = append(dnsDeleteParams, dnsDeleteParam{
			ZoneID: zoneID,
			recordID: p.getRecordID(records, cloudflare.DNSRecord{
				Name: deleteEndpoint.DNSName,
				Type: endpoint.RecordTypeCNAME,
			}),
		})
	}

	tunnelConfigParam := cloudflare.TunnelConfigurationParams{TunnelID: p.tunnelId, Config: oldConfigResult.Config}

	tunnelConfigParam.Config.Ingress = ingressConfigs.Get()
	tunnelConfigParam.Config.Ingress = append(tunnelConfigParam.Config.Ingress, catchAll)

	if p.DryRun {
		return nil
	}

	_, err = p.Client.UpdateTunnelConfiguration(ctx, cloudflare.AccountIdentifier(p.accountId), tunnelConfigParam)
	if err != nil {
		return fmt.Errorf("failed to update tunnel configs: %v", err)
	}
	log.Info("successfully update tunnel config")

	for _, createParam := range dnsCreateParams {
		zoneID := createParam.ZoneID
		_, err := p.Client.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), createParam)
		if err != nil {
			fmt.Printf("failed to create dns record: %v\n", err)
			continue
		}
		log.Info("successfully create record: ", createParam.Name)
	}

	for _, updateParam := range dnsUpdateParams {
		zoneID := updateParam.ZoneID
		_, err := p.Client.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), updateParam.cfup)
		if err != nil {
			fmt.Printf("failed to update dns record: %v\n", err)
			continue
		}
		log.Info("successfully update record: ", updateParam.cfup.Name)
	}

	for _, deleteParam := range dnsDeleteParams {
		zoneID := deleteParam.ZoneID
		err = p.Client.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), deleteParam.recordID)
		if err != nil {
			fmt.Printf("failed to delete dns record: %v\n", err)
			continue
		}
		log.Info("successfully delete record: ", deleteParam.recordID)
	}

	return nil
}

// Zones returns the list of hosted zones.
func (p *CloudFlareTunnelProvider) Zones(ctx context.Context) ([]cloudflare.Zone, error) {
	result := []cloudflare.Zone{}

	// if there is a zoneIDfilter configured
	// && if the filter isn't just a blank string (used in tests)
	if len(p.zoneIDFilter.ZoneIDs) > 0 && p.zoneIDFilter.ZoneIDs[0] != "" {
		log.Debugln("zoneIDFilter configured. only looking up zone IDs defined")
		for _, zoneID := range p.zoneIDFilter.ZoneIDs {
			log.Debugf("looking up zone %s", zoneID)
			detailResponse, err := p.Client.ZoneDetails(ctx, zoneID)
			if err != nil {
				log.Errorf("zone %s lookup failed, %v", zoneID, err)
				return result, err
			}
			log.WithFields(log.Fields{
				"zoneName": detailResponse.Name,
				"zoneID":   detailResponse.ID,
			}).Debugln("adding zone for consideration")
			result = append(result, detailResponse)
		}
		return result, nil
	}

	log.Debugln("no zoneIDFilter configured, looking at all zones")

	zonesResponse, err := p.Client.ListZonesContext(ctx)
	if err != nil {
		return nil, err
	}

	for _, zone := range zonesResponse.Result {
		if !p.domainFilter.Match(zone.Name) {
			log.Debugf("zone %s not in domain filter", zone.Name)
			continue
		}
		result = append(result, zone)
	}
	return result, nil
}

func (p CloudFlareTunnelProvider) updateZoneIdMapper(ctx context.Context) error {
	zones, err := p.Zones(ctx)
	if err != nil {
		return err
	}

	for _, z := range zones {
		p.zoneNameIDMapper.Add(z.ID, z.Name)
	}
	return nil
}

func (p *CloudFlareTunnelProvider) getRecordID(records []cloudflare.DNSRecord, record cloudflare.DNSRecord) string {
	for _, zoneRecord := range records {
		if zoneRecord.Name == record.Name && zoneRecord.Type == record.Type {
			return zoneRecord.ID
		}
	}
	return ""
}

// listDNSRecords performs automatic pagination of results on requests to cloudflare.ListDNSRecords with custom per_page values
func (p *CloudFlareTunnelProvider) listDNSRecordsWithAutoPagination(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
	var records []cloudflare.DNSRecord
	resultInfo := cloudflare.ResultInfo{PerPage: p.DNSRecordsPerPage, Page: 1}
	params := cloudflare.ListDNSRecordsParams{ResultInfo: resultInfo}
	for {
		pageRecords, resultInfo, err := p.Client.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), params)
		if err != nil {
			return nil, err
		}

		records = append(records, pageRecords...)
		params.ResultInfo = resultInfo.Next()
		if params.ResultInfo.Done() {
			break
		}
	}
	return records, nil
}

func convertHttps(target string) string {
	return fmt.Sprintf("https://%v:443", target)
}

// boolPtr is used as a helper function to return a pointer to a boolean
// Needed because some parameters require a pointer.
func boolPtr(b bool) *bool {
	return &b
}
