package utils

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestNow(t *testing.T) {
	g := NewWithT(t)
	dt := Now()
	g.Expect(dt.Time).To(Equal(dt.Time.Add(time.Duration(0) * time.Second)))

}

func TestDateTime_AddMethods(t *testing.T) {
	g := NewWithT(t)
	// Data de referência fixa para facilitar as asserções
	refTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	dt := DateTime{Time: refTime}

	t.Run("AddHours", func(t *testing.T) {
		res := dt.AddHours(2)
		g.Expect(res.Time).To(Equal(refTime.Add(2 * time.Hour)))
	})

	t.Run("AddMinutes", func(t *testing.T) {
		res := dt.AddMinutes(30)
		g.Expect(res.Time).To(Equal(refTime.Add(30 * time.Minute)))
	})

	t.Run("AddSeconds", func(t *testing.T) {
		res := dt.AddSeconds(45)
		g.Expect(res.Time).To(Equal(refTime.Add(45 * time.Second)))
	})
}

func TestDateTime_Formatting(t *testing.T) {
	g := NewWithT(t)
	refTime := time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC)
	dt := DateTime{Time: refTime}

	t.Run("ToString", func(t *testing.T) {
		// timeFormat é RFC3339
		g.Expect(dt.ToString()).To(Equal("2024-01-01T12:30:45Z"))
	})

	t.Run("Format", func(t *testing.T) {
		g.Expect(dt.Format("2006-01-02")).To(Equal("2024-01-01"))
	})
}

func TestGetTimeRemaining(t *testing.T) {
	g := NewWithT(t)

	// Teste com data futura
	nowDate := time.Now()
	future := nowDate.Add(1 * time.Hour)
	futureStr := future.Format(time.RFC3339)

	remaining := GetTimeRemaining(futureStr)
	// Deve ser aproximadamente 1 hora (margem de 5s para execução)
	g.Expect(remaining).To(BeNumerically("~", time.Hour, 5*time.Second))

	// Teste com data passada
	past := time.Now().Add(-1 * time.Hour)
	pastStr := past.Format(time.RFC3339)

	remainingPast := GetTimeRemaining(pastStr)
	g.Expect(remainingPast).To(BeNumerically("~", -time.Hour, 5*time.Second))
}

func TestNowIsAfterOrEqualCompareDate(t *testing.T) {
	g := NewWithT(t)

	// Data passada: Agora é DEPOIS da data -> True
	pastDate := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	g.Expect(NowIsAfterOrEqualCompareDate(pastDate)).To(BeTrue())

	// Data futura: Agora é ANTES da data -> False
	futureDate := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	g.Expect(NowIsAfterOrEqualCompareDate(futureDate)).To(BeFalse())
}
