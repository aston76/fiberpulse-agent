#ifndef MyAppVersion
  #define MyAppVersion "0.1.0-dev"
#endif

#define MyAppName "FiberPulse"
#define MyAppPublisher "FiberPulse Project"
#define MyAppExeName "fiberpulse.exe"

[Setup]
AppId={{7B4096BF-C93A-48D1-9094-E0799E4854AC}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\Programs\FiberPulse
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=dist
OutputBaseFilename=FiberPulse-{#MyAppVersion}-windows-x64-setup
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
CloseApplications=no
RestartApplications=no
UninstallDisplayIcon={app}\{#MyAppExeName}
VersionInfoVersion={#MyAppVersion}

[Files]
Source: "dist\fiberpulse.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\fiberpulse-updater.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\FiberPulse"; Filename: "{app}\{#MyAppExeName}"

[Run]
Filename: "{sys}\schtasks.exe"; Parameters: "/Create /F /SC ONLOGON /RL LIMITED /TN ""FiberPulse"" /TR ""\""{app}\{#MyAppExeName}\"""""; Flags: runhidden waituntilterminated
Filename: "{app}\{#MyAppExeName}"; Description: "Open FiberPulse"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\{#MyAppExeName}"; Parameters: "--quit"; Flags: runhidden waituntilterminated skipifdoesntexist
Filename: "{sys}\schtasks.exe"; Parameters: "/Delete /F /TN ""FiberPulse"""; Flags: runhidden waituntilterminated; RunOnceId: "RemoveFiberPulseLogonTask"

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
    DeleteFiberPulseData := MsgBox(
      'Delete local FiberPulse history, settings, reports and consent records?',
      mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES;

  if (CurUninstallStep = usPostUninstall) and DeleteFiberPulseData then
  begin
    DataDir := ExpandConstant('{localappdata}\FiberPulse');
    DelTree(DataDir, True, True, True);
  end;
end;
