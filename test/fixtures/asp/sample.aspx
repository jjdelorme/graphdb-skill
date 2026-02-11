<%@ Page Language="C#" %>
<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Hello World</h1>
    <%
        // This is a comment
        var x = 10;
        var y = 20;
    %>
    <script runat="server">
        public void MyMethod()
        {
            Console.WriteLine("Inside MyMethod");
        }

        protected int Calculate(int a, int b)
        {
            return a + b;
        }
    </script>
</body>
</html>
