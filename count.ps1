Get-ChildItem -Recurse -Filter *.go |
    Where-Object {
        $_.FullName -notmatch '\\(bin|\.git|\.zed|Markdowns|Refrences)\\'
    } |
    Get-Content |
    Where-Object { $_.Trim() -ne '' } |
    Measure-Object |
    Select-Object -ExpandProperty Count
