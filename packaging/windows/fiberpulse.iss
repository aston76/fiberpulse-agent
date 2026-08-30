#ifndef MyAppVersion
  #define MyAppVersion "0.1.0-dev"
#endif
#ifndef MyAppVersionInfo
  #define MyAppVersionInfo "0.1.0.0"
#endif

#define MyAppName "FiberPulse"
#define MyAppPublisher "FiberPulse"
#define MyAppExeName "fiberpulse.exe"

[Setup]
AppId={{7B4096BF-C93A-48D1-9094-E0799E4854AC}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL=https://fiberpulse.aston76.chatgpt.site/
AppSupportURL=https://fiberpulse.aston76.chatgpt.site/
AppUpdatesURL=https://github.com/aston76/fiberpulse-agent/releases/latest
DefaultDirName={localappdata}\Programs\FiberPulse
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.19041
SourceDir=..\..
OutputDir=dist
OutputBaseFilename=FiberPulse-{#MyAppVersion}-windows-x64-setup
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
SetupIconFile=packaging\windows\FiberPulse.ico
UninstallDisplayIcon={app}\FiberPulse.ico
UninstallDisplayName=FiberPulse
LicenseFile=LICENSE
CloseApplications=no
RestartApplications=no
SetupLogging=yes
VersionInfoVersion={#MyAppVersionInfo}
VersionInfoCompany={#MyAppPublisher}
VersionInfoDescription=FiberPulse connection quality monitor
VersionInfoProductName={#MyAppName}
VersionInfoProductVersion={#MyAppVersionInfo}
#ifdef FiberPulseReleaseSigning
SignTool=FiberPulseSign $f
SignedUninstaller=yes
#endif

[Files]
Source: "dist\fiberpulse.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\fiberpulse-updater.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "packaging\windows\FiberPulse.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\FiberPulse"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\FiberPulse.ico"

[Tasks]
Name: "autostart"; Description: "Start FiberPulse automatically when I sign in"; GroupDescription: "Startup:"; Flags: checkedonce

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "FiberPulse"; ValueData: """{app}\{#MyAppExeName}"" --background"; Flags: uninsdeletevalue; Tasks: autostart

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Open FiberPulse"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\{#MyAppExeName}"; Parameters: "--quit"; Flags: runhidden waituntilterminated skipifdoesntexist

[Code]
var
  DeleteFiberPulseData: Boolean;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  ResultCode: Integer;
begin
  Result := '';
  if FileExists(ExpandConstant('{app}\{#MyAppExeName}')) then
  begin
    if not Exec(ExpandConstant('{app}\{#MyAppExeName}'), '--quit', '', SW_HIDE,
      ewWaitUntilTerminated, ResultCode) then
      Result := 'FiberPulse could not be stopped safely. Close it and retry.'
    else if ResultCode <> 0 then
      Result := 'FiberPulse did not confirm a clean shutdown. Close it and retry.';
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  DataDir: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    if UninstallSilent then
      DeleteFiberPulseData := False
    else
      DeleteFiberPulseData := MsgBox(
        'Delete local FiberPulse history, settings, reports and consent records?',
        mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES;
  end;

  if (CurUninstallStep = usPostUninstall) and DeleteFiberPulseData then
  begin
    DataDir := ExpandConstant('{localappdata}\FiberPulse');
    DelTree(DataDir, True, True, True);
  end;
end;
