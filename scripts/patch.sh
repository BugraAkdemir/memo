sed -i 's/func (a \*App) GetRemoteAccessStatus() RemoteAccessStatus/func (a \*App) GetRemoteAccessStatus() interface{}/g' app.go
sed -i 's/func (a \*App) GetSyncAccount() SyncAccount/func (a \*App) GetSyncAccount() interface{}/g' app.go
sed -i 's/func (a \*App) GetSyncSettings() config.SyncConfig/func (a \*App) GetSyncSettings() interface{}/g' app.go
