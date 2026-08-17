; Memo — Windows Setup
; Inno Setup Script (https://jrsoftware.org/isinfo.php)
;
; Build:
;   C:\Program Files (x86)\Inno Setup 6\ISCC.exe installer.iss

#define MyAppName "Memo"
#define MyAppVersion "3.5.5"
#define MyAppPublisher "Bugra Akdemir"
#define MyAppURL "https://github.com/bugraakdemir/memo"
#define MyAppExeName "memo_flutter.exe"
#define MyAppAssocName "Memo Session"
#define MyAppAssocExt ".memo"
#define MyAppAssocKey StringChange(MyAppAssocName, " ", "") + MyAppAssocExt

[Setup]
; NOTE: The value of AppId uniquely identifies this application. Do not use the same AppId value in installers of other applications.
AppId={{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}
AppName={#MyAppName}
AppVerName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DisableProgramGroupPage=yes
LicenseFile=
InfoBeforeFile=
PrivilegesRequired=admin
OutputDir=build_output\dist
OutputBaseFilename=Memo-Setup-v{#MyAppVersion}
SetupIconFile=frontend\windows\runner\resources\app_icon.ico
UninstallDisplayIcon={app}\memo_flutter.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
DisableWelcomePage=no
DisableReadyPage=yes

[Languages]
Name: "turkish"; MessagesFile: "compiler:Languages\Turkish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: checkedonce

[Files]
Source: "build_output\stage\Memo\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Dirs]
Name: "{app}\data\models"
Name: "{app}\data\memory"
Name: "{app}\data\sessions"
Name: "{app}\data\agent-backups"
Name: "{commonappdata}\{#MyAppName}\data"; Permissions: users-full
Name: "{commonappdata}\{#MyAppName}\config"; Permissions: users-full

[UninstallDelete]
; Flutter's shared_preferences_windows plugin stores app-level UI prefs
; (setup wizard completed, language, theme, onboarding tour seen) under
; %APPDATA%\com.memo\<ProductName>, keyed by this exe's CompanyName/ProductName
; (see frontend/windows/runner/Runner.rc), independent of which Memo
; build/version is installed. Without this, those prefs silently survive a
; full uninstall+reinstall and the setup wizard never reappears on what
; looks like a fresh install.
Type: filesandordirs; Name: "{userappdata}\com.memo"

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\launch.vbs"; WorkingDir: "{app}"; IconFilename: "{app}\memo_flutter.exe"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\launch.vbs"; WorkingDir: "{app}"; Tasks: desktopicon; IconFilename: "{app}\memo_flutter.exe"

[Run]
; memo_flutter.exe needs the VC++ runtime (msvcp140.dll etc.), which a clean
; Windows install doesn't ship with. /install /quiet /norestart is safe to
; run every time — it's a no-op if an equal-or-newer runtime is already
; present, and won't prompt or reboot the machine.
Filename: "{app}\binaries\windows\vc_redist.x64.exe"; Parameters: "/install /quiet /norestart"; StatusMsg: "Installing Visual C++ Runtime..."; Flags: waituntilterminated skipifdoesntexist
Filename: "{app}\launch.vbs"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent shellexec; WorkingDir: "{app}"

[Registry]
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocExt}\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Flags: uninsdeletevalue
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}"; ValueType: string; ValueName: ""; ValueData: "{#MyAppAssocName}"; Flags: uninsdeletekey
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\memo_flutter.exe,0"
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\memo_flutter.exe"" ""%1"""
Root: HKA; Subkey: "Software\Classes\Applications\memo_flutter.exe\SupportedTypes"; ValueType: string; ValueName: ".memo"; ValueData: ""

[Code]
const
  EnvironmentKey = 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment';

// Adds Path to the machine-wide PATH so `memo.exe` (shipped in {app}) is
// reachable by typing `memo` in a new cmd/PowerShell window. Skips silently
// if it's already there (repeat installs/upgrades).
procedure EnvAddPath(Path: string);
var
  Paths: string;
begin
  if not RegQueryStringValue(HKEY_LOCAL_MACHINE, EnvironmentKey, 'Path', Paths) then
    Paths := '';

  if Pos(';' + Uppercase(Path) + ';', ';' + Uppercase(Paths) + ';') > 0 then
    exit;

  if (Length(Paths) > 0) and (Paths[Length(Paths)] <> ';') then
    Paths := Paths + ';';
  Paths := Paths + Path + ';';

  if RegWriteStringValue(HKEY_LOCAL_MACHINE, EnvironmentKey, 'Path', Paths) then
    Log(Format('Added [%s] to PATH', [Path]))
  else
    Log(Format('Failed to add [%s] to PATH', [Path]));
end;

// Removes Path from the machine-wide PATH on uninstall.
procedure EnvRemovePath(Path: string);
var
  Paths: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKEY_LOCAL_MACHINE, EnvironmentKey, 'Path', Paths) then
    exit;

  P := Pos(';' + Uppercase(Path) + ';', ';' + Uppercase(Paths) + ';');
  if P = 0 then
    exit;

  Delete(Paths, P, Length(Path) + 1);

  if RegWriteStringValue(HKEY_LOCAL_MACHINE, EnvironmentKey, 'Path', Paths) then
    Log(Format('Removed [%s] from PATH', [Path]))
  else
    Log(Format('Failed to remove [%s] from PATH', [Path]));
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  BaseDir: String;
  DataDir: String;
  ConfigDir: String;
begin
  if CurStep = ssPostInstall then
  begin
    BaseDir := ExpandConstant('{commonappdata}') + '\Memo';
    DataDir := BaseDir + '\data';
    ConfigDir := BaseDir + '\config';
    if not DirExists(DataDir) then
    begin
      CreateDir(DataDir);
      CreateDir(DataDir + '\models');
      CreateDir(DataDir + '\memory');
      CreateDir(DataDir + '\sessions');
      CreateDir(DataDir + '\agent-backups');
    end;

    if not FileExists(DataDir + '\permissions.json') then
    begin
      SaveStringToFile(DataDir + '\permissions.json', '[]', False);
    end;

    // Seed config.yaml into the writable ProgramData location so the app
    // (which reads/writes config from there) starts with packaged defaults.
    if not DirExists(ConfigDir) then
      CreateDir(ConfigDir);
    if (not FileExists(ConfigDir + '\config.yaml')) and FileExists(ExpandConstant('{app}') + '\config\config.yaml') then
    begin
      FileCopy(ExpandConstant('{app}') + '\config\config.yaml', ConfigDir + '\config.yaml', False);
    end;

    // So typing `memo` in a terminal works without the user touching PATH.
    EnvAddPath(ExpandConstant('{app}'));
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    EnvRemovePath(ExpandConstant('{app}'));
end;
