@echo off

::协议文件路径, 最后不要跟“\”符号
set SOURCE_FOLDER=.\clientpoto

::C#编译器路径
set CS_COMPILER_PATH=.\protobuf-net\ProtoGen\protogen.exe
::C#文件生成路径, 最后不要跟“\”符号
set CS_TARGET_PATH=.\csproto

::删除之前创建的文件
del %CS_TARGET_PATH%\*.* /f /s /q

::遍历所有文件
for /f "delims=" %%i in ('dir /b "%SOURCE_FOLDER%\*.proto"') do (
    
    ::生成 C# 代码
    echo %CS_COMPILER_PATH% -i:%%i -o:%CS_TARGET_PATH%\%%~ni.cs
    %CS_COMPILER_PATH% -i:%%i -o:%CS_TARGET_PATH%\%%~ni.cs
    
)

echo 协议生成完毕。

pause