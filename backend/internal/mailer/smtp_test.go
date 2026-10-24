package mailer

import "testing"

func TestEnvelopeAddress(t *testing.T) {
	cases := map[string]string{
		"root@codefever.cn":                "root@codefever.cn",
		"CodeFever <root@codefever.cn>":    "root@codefever.cn",
		"  CodeFever <root@codefever.cn> ": "root@codefever.cn",
	}
	for in, want := range cases {
		if got := envelopeAddress(in); got != want {
			t.Fatalf("envelopeAddress(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestValidateHeaderFieldRejectsInjection(t *testing.T) {
	if err := validateHeaderField("Subject", "ok\r\nBcc: evil@example.com"); err == nil {
		t.Fatal("CRLF 注入必须被拒绝")
	}
	if err := validateHeaderField("To", ""); err == nil {
		t.Fatal("空字段必须被拒绝")
	}
	if err := validateHeaderField("To", "a@b.c"); err != nil {
		t.Fatalf("正常字段被拒绝: %v", err)
	}
}

func TestFromEnvFallsBackToLocalMailer(t *testing.T) {
	t.Setenv("CODEFEVER_SMTP_HOST", "")
	if _, ok := FromEnv("root@codefever.cn").(*LocalMailer); !ok {
		t.Fatal("未配置 SMTP 时应回退 LocalMailer")
	}
}

func TestFromEnvSMTP(t *testing.T) {
	t.Setenv("CODEFEVER_SMTP_HOST", "smtp.example.com")
	t.Setenv("CODEFEVER_SMTP_PORT", "465")
	m, ok := FromEnv("root@codefever.cn").(*SMTPMailer)
	if !ok {
		t.Fatal("配置 SMTP 后应返回 SMTPMailer")
	}
	if !m.ImplicitTLS || m.Port != 465 {
		t.Fatalf("默认 465 应按 implicit TLS 处理: %+v", m)
	}
}
