Public Class Greeter
    Public Sub Greet(name As String)
        Console.WriteLine("Hello " & name)
        Calculate()
    End Sub
    
    Function Calculate() As Integer
        Return 1
    End Function
End Class
