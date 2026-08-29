package dnsserver

import (
	"database/sql"
	"log"
	"net"
	"strings"

	cdns "github.com/Ribco/clicapot/internal/dns"
	"github.com/miekg/dns"
)

type Server struct {
	db   *sql.DB
	addr string
}

func New(db *sql.DB, addr string) *Server {
	return &Server{
		db:   db,
		addr: addr,
	}
}

func (s *Server) Start() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleDNS)

	udp := &dns.Server{
		Addr:    s.addr,
		Net:     "udp",
		Handler: mux,
	}

	tcp := &dns.Server{
		Addr:    s.addr,
		Net:     "tcp",
		Handler: mux,
	}

	go func() {
		log.Printf("DNS UDP listening on %s", s.addr)
		if err := udp.ListenAndServe(); err != nil {
			log.Printf("DNS UDP stopped: %v", err)
		}
	}()

	go func() {
		log.Printf("DNS TCP listening on %s", s.addr)
		if err := tcp.ListenAndServe(); err != nil {
			log.Printf("DNS TCP stopped: %v", err)
		}
	}()

	return nil
}

func (s *Server) handleDNS(w dns.ResponseWriter, req *dns.Msg) {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = true
	resp.RecursionAvailable = false

	if len(req.Question) == 0 {
		resp.Rcode = dns.RcodeFormatError
		_ = w.WriteMsg(resp)
		return
	}

	q := req.Question[0]
	name := strings.ToLower(strings.TrimSuffix(q.Name, "."))

	zone, err := s.findZone(name)
	if err != nil {
		resp.Rcode = dns.RcodeNameError
		_ = w.WriteMsg(resp)
		return
	}

	records, err := cdns.ListAllRecords(s.db, zone.ID)
	if err != nil {
		resp.Rcode = dns.RcodeServerFailure
		_ = w.WriteMsg(resp)
		return
	}

	for _, record := range records {
		if !recordMatches(record, name, zone.Name) {
			continue
		}

		rr := makeRR(record, zone.Name)
		if rr == nil {
			continue
		}

		switch q.Qtype {
		case dns.TypeA:
			if record.Type == "A" {
				resp.Answer = append(resp.Answer, rr)
			}
		case dns.TypeAAAA:
			if record.Type == "AAAA" {
				resp.Answer = append(resp.Answer, rr)
			}
		case dns.TypeCNAME:
			if record.Type == "CNAME" {
				resp.Answer = append(resp.Answer, rr)
			}
		case dns.TypeMX:
			if record.Type == "MX" {
				resp.Answer = append(resp.Answer, rr)
			}
		case dns.TypeTXT:
			if record.Type == "TXT" {
				resp.Answer = append(resp.Answer, rr)
			}
		case dns.TypeNS:
			if record.Type == "NS" {
				resp.Answer = append(resp.Answer, rr)
			}
		case dns.TypeANY:
			resp.Answer = append(resp.Answer, rr)
		}
	}

	if len(resp.Answer) == 0 {
		resp.Rcode = dns.RcodeNameError
	}

	_ = w.WriteMsg(resp)
}

func (s *Server) findZone(name string) (cdns.Zone, error) {
	zones, err := cdns.ListAllZones(s.db)
	if err != nil {
		return cdns.Zone{}, err
	}

	for _, zone := range zones {
		if name == zone.Name || strings.HasSuffix(name, "."+zone.Name) {
			return zone, nil
		}
	}

	return cdns.Zone{}, cdns.ErrNotFound
}

func recordMatches(r cdns.Record, name, zone string) bool {
	recordName := strings.TrimSuffix(strings.ToLower(r.Name), ".")

	if recordName == "@" || recordName == "" {
		return name == zone
	}

	if strings.HasSuffix(recordName, "."+zone) {
		return name == recordName
	}

	return name == recordName+"."+zone
}

func makeRR(r cdns.Record, zone string) dns.RR {
	name := strings.TrimSuffix(strings.ToLower(r.Name), ".")

	if name == "" || name == "@" {
		name = zone
	} else if !strings.HasSuffix(name, "."+zone) {
		name += "." + zone
	}

	name += "."

	header := dns.RR_Header{
		Name:   name,
		Rrtype: dns.StringToType[r.Type],
		Class:  dns.ClassINET,
		Ttl:    uint32(r.TTL),
	}

	switch r.Type {
	case "A":
		ip := net.ParseIP(r.Content)
		if ip == nil {
			return nil
		}
		return &dns.A{
			Hdr: header,
			A:   ip.To4(),
		}

	case "AAAA":
		ip := net.ParseIP(r.Content)
		if ip == nil {
			return nil
		}
		return &dns.AAAA{
			Hdr:  header,
			AAAA: ip,
		}

	case "CNAME":
		return &dns.CNAME{
			Hdr:    header,
			Target: dns.Fqdn(r.Content),
		}

	case "NS":
		return &dns.NS{
			Hdr: header,
			Ns:  dns.Fqdn(r.Content),
		}

	case "TXT":
		return &dns.TXT{
			Hdr: header,
			Txt: []string{r.Content},
		}

	case "MX":
		priority := uint16(10)
		if r.Priority != nil && *r.Priority >= 0 {
			priority = uint16(*r.Priority)
		}

		return &dns.MX{
			Hdr:        header,
			Preference: priority,
			Mx:         dns.Fqdn(r.Content),
		}
	}

	return nil
}
