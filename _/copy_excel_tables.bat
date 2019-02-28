set dest_path=%cd%\..\game_excel
md %dest_path%
cd D:\work\IHProject\Design\ÊýÖµ\gameres
"C:\Program Files\TortoiseSVN\bin\TortoiseProc.exe" /command:update /path:"./"
copy *.xlsx %dest_path%