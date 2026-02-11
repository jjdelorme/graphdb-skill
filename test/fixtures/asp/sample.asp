<%@ Language="VBScript" %>
<html>
<body>
<%
    Dim name
    name = "World"
%>
<script runat="server">
    Sub MySub()
        Response.Write("Hello")
    End Sub

    Function Add(a, b)
        Add = a + b
    End Function
</script>
</body>
</html>
