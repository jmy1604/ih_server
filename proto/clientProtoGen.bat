@echo off

::Э���ļ�·��, ���Ҫ����\������
set SOURCE_FOLDER=.\clientpoto

::C#������·��
set CS_COMPILER_PATH=.\protobuf-net\ProtoGen\protogen.exe
::C#�ļ�����·��, ���Ҫ����\������
set CS_TARGET_PATH=.\csproto

::ɾ��֮ǰ�������ļ�
del %CS_TARGET_PATH%\*.* /f /s /q

::���������ļ�
for /f "delims=" %%i in ('dir /b "%SOURCE_FOLDER%\*.proto"') do (
    
    ::���� C# ����
    echo %CS_COMPILER_PATH% -i:%%i -o:%CS_TARGET_PATH%\%%~ni.cs
    %CS_COMPILER_PATH% -i:%%i -o:%CS_TARGET_PATH%\%%~ni.cs
    
)

echo Э��������ϡ�

pause