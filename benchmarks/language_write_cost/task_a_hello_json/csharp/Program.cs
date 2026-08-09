using System.Net;
using System.Text;
using System.Text.Json;

var listener = new HttpListener();
listener.Prefixes.Add("http://localhost:8080/");
listener.Start();

while (true)
{
    var ctx = listener.GetContext();
    var res = ctx.Response;
    byte[] buffer;

    if (ctx.Request.Url?.AbsolutePath == "/json")
    {
        var msg = new { status = "success", message = "Hello from HowlFrame JSON endpoint!" };
        buffer = Encoding.UTF8.GetBytes(JsonSerializer.Serialize(msg));
        res.ContentType = "application/json";
    }
    else
    {
        buffer = Encoding.UTF8.GetBytes("Hello, World! HowlFrame language is alive!");
        res.ContentType = "text/plain";
    }

    res.ContentLength64 = buffer.Length;
    res.OutputStream.Write(buffer, 0, buffer.Length);
    res.OutputStream.Close();
}
