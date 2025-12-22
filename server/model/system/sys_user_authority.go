package system

// SysUserAuthority User and authority relationship table structure
type SysUserAuthority struct {
	SysUserId               uint `gorm:"column:sys_user_id"`
	SysAuthorityAuthorityId uint `gorm:"column:sys_authority_authority_id"`
}

func (s *SysUserAuthority) TableName() string {
	return "sys_user_authority"
}
