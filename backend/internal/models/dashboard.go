package models

// DashboardResponse represents aggregated dashboard data
type DashboardResponse struct {
	UserID        int                 `json:"user_id"`
	TotalBanks    int                 `json:"total_banks"`
	ActiveBanks   int                 `json:"active_banks"`
	TotalAccounts int                 `json:"total_accounts"`
	Banks         []BankDashboardInfo `json:"banks"`
}

// BankDashboardInfo represents bank info for dashboard
type BankDashboardInfo struct {
	Provider      BankProvider `json:"provider"`
	Status        string       `json:"status"`
	AccountsCount int          `json:"accounts_count"`
	Accounts      []*Account   `json:"accounts,omitempty"`
	Error         string       `json:"error,omitempty"` // If failed to fetch
}
